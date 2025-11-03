package server

import (
	"context"
	"fmt"
	"io"
	"log"
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
	
	log.Printf("[DEBUG] New connection accepted")
	
	p, err := parser.NewParser(conn)
	if err != nil {
		log.Printf("[ERROR] Failed to create parser: %v", err)
		s.writeError(conn, "parser", "invalid header", err.Error())
		return
	}
	
	for {
		cmd, err := p.ParseCommand()
		if err == io.EOF {
			log.Printf("[DEBUG] Connection closed by client")
			break
		}
		if err != nil {
			log.Printf("[ERROR] Parse error: %v", err)
			s.writeError(conn, "parser", "parse error", err.Error())
			continue
		}
		
		log.Printf("[DEBUG] Executing command: %s with %d args", cmd.Name, len(cmd.Args))
		s.executeCommand(conn, cmd)
	}
}

func (s *Server) executeCommand(conn net.Conn, cmd *parser.Command) {
	switch cmd.Name {
	case "+filter-name":
		s.handleFilterName(conn, cmd)
	case "+filter-cat":
		s.handleFilterCat(conn, cmd)
	case "+filter-path":
		s.handleFilterPath(conn, cmd)
	case "0filters":
		s.handleResetFilters(conn)
	case "list":
		s.handleList(conn)
	case "run":
		s.handleRun(conn, cmd)
	case "lang":
		s.handleLang(conn, cmd)
	default:
		s.writeError(conn, cmd.Name, "unknown command", "Command not recognized")
	}
}

func (s *Server) handleFilterName(conn net.Conn, cmd *parser.Command) {
	log.Printf("[DEBUG] Handling filter-name command")
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
		log.Printf("[DEBUG] Added name filter: %v (op: %s)", expr.Values, expr.Op)
	}
	
	// Send success response
	attrs := fmt.Sprintf("cmd: +filter-name\nstatus: 0\n\n")
	s.writeResponse(conn, attrs)
}

func (s *Server) handleFilterCat(conn net.Conn, cmd *parser.Command) {
	log.Printf("[DEBUG] Handling filter-cat command")
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
		log.Printf("[DEBUG] Added cat filter: %v (op: %s)", expr.Values, expr.Op)
	}
	
	// Send success response
	attrs := fmt.Sprintf("cmd: +filter-cat\nstatus: 0\n\n")
	s.writeResponse(conn, attrs)
}

func (s *Server) handleFilterPath(conn net.Conn, cmd *parser.Command) {
	log.Printf("[DEBUG] Handling filter-path command")
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
		log.Printf("[DEBUG] Added path filter: %v (op: %s)", expr.Values, expr.Op)
	}
	
	// Send success response
	attrs := fmt.Sprintf("cmd: +filter-path\nstatus: 0\n\n")
	s.writeResponse(conn, attrs)
}

func (s *Server) handleResetFilters(conn net.Conn) {
	log.Printf("[DEBUG] Resetting all filters")
	s.filters.mu.Lock()
	defer s.filters.mu.Unlock()
	s.filters.nameFilters = []FilterExpr{}
	s.filters.catFilters = []FilterExpr{}
	s.filters.pathFilters = []FilterExpr{}
	
	// Send success response
	attrs := fmt.Sprintf("cmd: 0filters\nstatus: 0\n\n")
	s.writeResponse(conn, attrs)
}

func (s *Server) handleList(conn net.Conn) {
	log.Printf("[DEBUG] Handling list command")
	
	idx := s.indexer.GetIndex()
	allEntries := idx.GetAll()
	
	s.filters.mu.RLock()
	filtered := s.filterEntries(allEntries)
	s.filters.mu.RUnlock()
	
	log.Printf("[DEBUG] Found %d entries after filtering (total: %d)", len(filtered), len(allEntries))
	
	attrs := fmt.Sprintf("list-len: %d\npages: 1\n\n", len(filtered))
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
	
	s.writeResponse(conn, attrs+body.String())
	log.Printf("[DEBUG] List response sent")
}

func (s *Server) handleRun(conn net.Conn, cmd *parser.Command) {
	log.Printf("[DEBUG] Handling run command")
	
	if len(cmd.Args) == 0 || cmd.Args[0].Type != parser.TypeInt {
		log.Printf("[ERROR] Run command missing id parameter")
		s.writeError(conn, "run", "missing id", "run command requires an id parameter")
		return
	}
	
	id := cmd.Args[0].Int
	log.Printf("[DEBUG] Running application with id: %d", id)
	
	idx := s.indexer.GetIndex()
	entry, ok := idx.Get(int64(id))
	if !ok {
		log.Printf("[ERROR] Index %d not found", id)
		s.writeError(conn, "run", "index not found", "Can't run application, requested index not found.")
		return
	}
	
	log.Printf("[DEBUG] Found entry: %s, exec: %s, terminal: %v", entry.Name, entry.Exec, entry.Terminal)
	
	// Execute the command
	var execCmd *exec.Cmd
	if entry.Terminal {
		cfg := config.Get()
		term := cfg.Terminal()
		execCmd = exec.Command(term, "-e", entry.Exec)
		log.Printf("[DEBUG] Executing in terminal: %s -e %s", term, entry.Exec)
	} else {
		// Parse exec command
		parts := strings.Fields(entry.Exec)
		if len(parts) == 0 {
			log.Printf("[ERROR] Empty exec command")
			s.writeError(conn, "run", "invalid exec", "Empty exec command")
			return
		}
		execCmd = exec.Command(parts[0], parts[1:]...)
		log.Printf("[DEBUG] Executing: %v", parts)
	}
	
	err := execCmd.Start()
	if err != nil {
		log.Printf("[ERROR] Failed to start command: %v", err)
		s.writeError(conn, "run", "execution failed", err.Error())
		return
	}
	
	pid := execCmd.Process.Pid
	log.Printf("[DEBUG] Command started successfully with PID: %d", pid)
	
	attrs := fmt.Sprintf("cmd: run\nidx: %d\nstatus: 0\npid: %d\n\n", id, pid)
	s.writeResponse(conn, attrs)
	log.Printf("[DEBUG] Run response sent")
}

func (s *Server) handleLang(conn net.Conn, cmd *parser.Command) {
	log.Printf("[DEBUG] Handling lang command")
	if len(cmd.Args) == 0 || cmd.Args[0].Type != parser.TypeString {
		log.Printf("[WARN] Lang command missing string parameter")
		s.writeError(conn, "lang", "missing parameter", "lang command requires a string parameter")
		return
	}
	s.lang = cmd.Args[0].Str
	log.Printf("[DEBUG] Language set to: %s", s.lang)
	
	// Send success response
	attrs := fmt.Sprintf("cmd: lang\nstatus: 0\nlang: %s\n\n", s.lang)
	s.writeResponse(conn, attrs)
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

// writeResponse writes a response with TXT01 header
func (s *Server) writeResponse(conn net.Conn, response string) {
	log.Printf("[DEBUG] Writing response (length: %d bytes)", len(response))
	header := []byte("TXT01")
	n, err := conn.Write(header)
	if err != nil {
		log.Printf("[ERROR] Failed to write header: %v", err)
		return
	}
	if n != len(header) {
		log.Printf("[ERROR] Partial header write: %d/%d bytes", n, len(header))
		return
	}
	
	n, err = conn.Write([]byte(response))
	if err != nil {
		log.Printf("[ERROR] Failed to write response body: %v", err)
		return
	}
	log.Printf("[DEBUG] Response written successfully: %d bytes", n)
}

func (s *Server) writeError(conn net.Conn, cmd, errType, desc string) {
	log.Printf("[ERROR] Writing error response: cmd=%s, type=%s, desc=%s", cmd, errType, desc)
	errorMsg := fmt.Sprintf("error-cmd: %s\nerror: %s\ndesc: %s\n\n", cmd, errType, desc)
	s.writeResponse(conn, errorMsg)
}
