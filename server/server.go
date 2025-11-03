package server

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/0xADE/ade-ctld/internal/config"
	"github.com/0xADE/ade-ctld/internal/indexer"
	"github.com/0xADE/ade-ctld/parser"
)

// Server handles Unix socket connections and command execution
type Server struct {
	listener net.Listener
	indexer  *indexer.Indexer
	running  bool
	mu       sync.RWMutex
	filters  *Filters
	lang     string
}

// Filters stores current filter settings
type Filters struct {
	mu          sync.RWMutex
	nameFilters []FilterExpr
	catFilters  []FilterExpr
	pathFilters []FilterExpr
}

// FilterExpr represents a filter expression
type FilterExpr struct {
	Values []string
	Op     string // "or", "and", "not"
}

// NewServer creates a new server instance
func NewServer(idx *indexer.Indexer) (*Server, error) {
	cfg := config.Get()
	socketPath := cfg.UnixSocket()
	
	// Create directory if needed
	socketDir := filepath.Dir(socketPath)
	if err := os.MkdirAll(socketDir, 0755); err != nil {
		return nil, err
	}
	
	// Remove existing socket if it exists
	os.Remove(socketPath)
	
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, err
	}
	
	return &Server{
		listener: listener,
		indexer:  idx,
		filters:  &Filters{},
		lang:     "en",
	}, nil
}

// Start starts the server
func (s *Server) Start(ctx context.Context) error {
	s.mu.Lock()
	s.running = true
	s.mu.Unlock()
	
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		
		conn, err := s.listener.Accept()
		if err != nil {
			s.mu.RLock()
			running := s.running
			s.mu.RUnlock()
			if !running {
				return nil
			}
			continue
		}
		
		go s.handleConnection(conn)
	}
}

// Stop stops the server
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.running = false
	return s.listener.Close()
}

func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()
	
	p, err := parser.NewParser(conn)
	if err != nil {
		s.writeError(conn, "parser", "invalid header", err.Error())
		return
	}
	
	for {
		cmd, err := p.ParseCommand()
		if err == io.EOF {
			break
		}
		if err != nil {
			s.writeError(conn, "parser", "parse error", err.Error())
			continue
		}
		
		s.executeCommand(conn, cmd)
	}
}

func (s *Server) executeCommand(conn net.Conn, cmd *parser.Command) {
	switch cmd.Name {
	case "+filter-name":
		s.handleFilterName(cmd)
	case "+filter-cat":
		s.handleFilterCat(cmd)
	case "+filter-path":
		s.handleFilterPath(cmd)
	case "0filters":
		s.handleResetFilters()
	case "list":
		s.handleList(conn)
	case "run":
		s.handleRun(conn, cmd)
	case "lang":
		s.handleLang(cmd)
	default:
		s.writeError(conn, cmd.Name, "unknown command", "Command not recognized")
	}
}

func (s *Server) handleFilterName(cmd *parser.Command) {
	s.filters.mu.Lock()
	defer s.filters.mu.Unlock()
	
	expr := FilterExpr{Values: []string{}, Op: "or"}
	for _, arg := range cmd.Args {
		if arg.Type == parser.TypeString {
			expr.Values = append(expr.Values, arg.Str)
		} else if arg.Type == parser.TypeBool {
			if arg.Bool {
				expr.Op = "or"
			} else {
				expr.Op = "and"
			}
		}
	}
	
	if len(expr.Values) > 0 {
		s.filters.nameFilters = append(s.filters.nameFilters, expr)
	}
}

func (s *Server) handleFilterCat(cmd *parser.Command) {
	s.filters.mu.Lock()
	defer s.filters.mu.Unlock()
	
	expr := FilterExpr{Values: []string{}, Op: "and"}
	for _, arg := range cmd.Args {
		if arg.Type == parser.TypeString {
			expr.Values = append(expr.Values, arg.Str)
		} else if arg.Type == parser.TypeBool {
			if arg.Bool {
				expr.Op = "or"
			} else {
				expr.Op = "and"
			}
		}
	}
	
	if len(expr.Values) > 0 {
		s.filters.catFilters = append(s.filters.catFilters, expr)
	}
}

func (s *Server) handleFilterPath(cmd *parser.Command) {
	s.filters.mu.Lock()
	defer s.filters.mu.Unlock()
	
	expr := FilterExpr{Values: []string{}, Op: "or"}
	for _, arg := range cmd.Args {
		if arg.Type == parser.TypeString {
			expr.Values = append(expr.Values, arg.Str)
		} else if arg.Type == parser.TypeBool {
			if arg.Bool {
				expr.Op = "or"
			} else {
				expr.Op = "and"
			}
		}
	}
	
	if len(expr.Values) > 0 {
		s.filters.pathFilters = append(s.filters.pathFilters, expr)
	}
}

func (s *Server) handleResetFilters() {
	s.filters.mu.Lock()
	defer s.filters.mu.Unlock()
	s.filters.nameFilters = []FilterExpr{}
	s.filters.catFilters = []FilterExpr{}
	s.filters.pathFilters = []FilterExpr{}
}

func (s *Server) handleList(conn net.Conn) {
	idx := s.indexer.GetIndex()
	allEntries := idx.GetAll()
	
	s.filters.mu.RLock()
	filtered := s.filterEntries(allEntries)
	s.filters.mu.RUnlock()
	
	attrs := fmt.Sprintf("list-len: %d\npages: 1\n\n", len(filtered))
	conn.Write([]byte(attrs))
	
	body := strings.Builder{}
	for _, entry := range filtered {
		name := entry.Name
		if s.lang != "" && entry.Names != nil {
			if locName, ok := entry.Names[s.lang]; ok {
				name = locName
			}
		}
		body.WriteString(fmt.Sprintf("%d %s\n", entry.ID, name))
	}
	
	conn.Write([]byte(body.String()))
}

func (s *Server) handleRun(conn net.Conn, cmd *parser.Command) {
	if len(cmd.Args) == 0 || cmd.Args[0].Type != parser.TypeInt {
		s.writeError(conn, "run", "missing id", "run command requires an id parameter")
		return
	}
	
	id := cmd.Args[0].Int
	idx := s.indexer.GetIndex()
	entry, ok := idx.Get(int64(id))
	if !ok {
		s.writeError(conn, "run", "index not found", "Can't run application, requested index not found.")
		return
	}
	
	// Execute the command
	var execCmd *exec.Cmd
	if entry.Terminal {
		cfg := config.Get()
		term := cfg.Terminal()
		execCmd = exec.Command(term, "-e", entry.Exec)
	} else {
		// Parse exec command
		parts := strings.Fields(entry.Exec)
		if len(parts) == 0 {
			s.writeError(conn, "run", "invalid exec", "Empty exec command")
			return
		}
		execCmd = exec.Command(parts[0], parts[1:]...)
	}
	
	err := execCmd.Start()
	if err != nil {
		s.writeError(conn, "run", "execution failed", err.Error())
		return
	}
	
	pid := execCmd.Process.Pid
	attrs := fmt.Sprintf("cmd: run\nidx: %d\nstatus: 0\npid: %d\n\n", id, pid)
	conn.Write([]byte(attrs))
}

func (s *Server) handleLang(cmd *parser.Command) {
	if len(cmd.Args) == 0 || cmd.Args[0].Type != parser.TypeString {
		return
	}
	s.lang = cmd.Args[0].Str
}

func (s *Server) filterEntries(entries []*indexer.Entry) []*indexer.Entry {
	var result []*indexer.Entry
	
	for _, entry := range entries {
		if s.matchesFilters(entry) {
			result = append(result, entry)
		}
	}
	
	return result
}

func (s *Server) matchesFilters(entry *indexer.Entry) bool {
	// Check name filters
	if len(s.filters.nameFilters) > 0 {
		matched := false
		for _, filter := range s.filters.nameFilters {
			if s.matchesNameFilter(entry, filter) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	
	// Check category filters
	if len(s.filters.catFilters) > 0 {
		matched := false
		for _, filter := range s.filters.catFilters {
			if s.matchesCatFilter(entry, filter) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	
	// Check path filters
	if len(s.filters.pathFilters) > 0 {
		matched := false
		for _, filter := range s.filters.pathFilters {
			if s.matchesPathFilter(entry, filter) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	
	return true
}

func (s *Server) matchesNameFilter(entry *indexer.Entry, filter FilterExpr) bool {
	searchText := strings.ToLower(entry.Name)
	for _, value := range filter.Values {
		if strings.Contains(searchText, strings.ToLower(value)) {
			return true
		}
	}
	// Also check localized names
	for _, name := range entry.Names {
		searchText := strings.ToLower(name)
		for _, value := range filter.Values {
			if strings.Contains(searchText, strings.ToLower(value)) {
				return true
			}
		}
	}
	return false
}

func (s *Server) matchesCatFilter(entry *indexer.Entry, filter FilterExpr) bool {
	for _, cat := range entry.Categories {
		for _, filterCat := range filter.Values {
			if strings.EqualFold(cat, filterCat) {
				return true
			}
		}
	}
	return false
}

func (s *Server) matchesPathFilter(entry *indexer.Entry, filter FilterExpr) bool {
	for _, filterPath := range filter.Values {
		if strings.Contains(entry.Path, filterPath) {
			return true
		}
	}
	return false
}

func (s *Server) writeError(conn net.Conn, cmd, errType, desc string) {
	errorMsg := fmt.Sprintf("error-cmd: %s\nerror: %s\ndesc: %s\n\n", cmd, errType, desc)
	conn.Write([]byte(errorMsg))
}
