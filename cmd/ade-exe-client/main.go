package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"strings"

	"github.com/0xADE/ade-ctld/internal/config"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <command> [args...]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Commands:\n")
		fmt.Fprintf(os.Stderr, "  list                 - List all applications\n")
		fmt.Fprintf(os.Stderr, "  filter-name <name>   - Filter by name\n")
		fmt.Fprintf(os.Stderr, "  filter-cat <cat>     - Filter by category\n")
		fmt.Fprintf(os.Stderr, "  reset-filters        - Reset all filters\n")
		fmt.Fprintf(os.Stderr, "  run <id>             - Run application by ID\n")
		fmt.Fprintf(os.Stderr, "  lang <locale>        - Set language\n")
		fmt.Fprintf(os.Stderr, "  interactive          - Interactive mode\n")
		os.Exit(1)
	}
	
	// Initialize config to get socket path
	if err := config.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize config: %v\n", err)
		os.Exit(1)
	}
	
	cfg := config.Get()
	socketPath := cfg.UnixSocket()
	
	// Expand socket path
	if strings.HasPrefix(socketPath, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get home directory: %v\n", err)
			os.Exit(1)
		}
		socketPath = strings.Replace(socketPath, "~", home, 1)
	}
	
	// Connect to socket
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to socket %s: %v\n", socketPath, err)
		os.Exit(1)
	}
	defer conn.Close()
	
	cmd := os.Args[1]
	
	if cmd == "interactive" {
		runInteractive(conn)
		return
	}
	
	// Send header
	conn.Write([]byte("TXT01"))
	
	// Execute command
	switch cmd {
	case "list":
		sendCommand(conn, "list", nil)
	case "filter-name":
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "Usage: %s filter-name <name>\n", os.Args[0])
			os.Exit(1)
		}
		sendCommand(conn, "+filter-name", []string{os.Args[2]})
	case "filter-cat":
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "Usage: %s filter-cat <category>\n", os.Args[0])
			os.Exit(1)
		}
		sendCommand(conn, "+filter-cat", []string{os.Args[2]})
	case "reset-filters":
		sendCommand(conn, "0filters", nil)
	case "run":
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "Usage: %s run <id>\n", os.Args[0])
			os.Exit(1)
		}
		sendCommand(conn, "run", []string{os.Args[2]})
	case "lang":
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "Usage: %s lang <locale>\n", os.Args[0])
			os.Exit(1)
		}
		sendCommand(conn, "lang", []string{os.Args[2]})
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		os.Exit(1)
	}
	
	// Read and print response
	readResponse(conn)
}

func sendCommand(conn net.Conn, cmdName string, args []string) {
	// Send header if not already sent
	conn.Write([]byte("TXT01"))
	
	// Send arguments
	for _, arg := range args {
		conn.Write([]byte(fmt.Sprintf(`"%s`, arg)))
		conn.Write([]byte{'\n'})
	}
	
	// Send command
	conn.Write([]byte(cmdName))
	conn.Write([]byte{'\n'})
}

func readResponse(conn net.Conn) {
	reader := bufio.NewReader(conn)
	
	// Read header
	header := make([]byte, 5)
	io.ReadFull(reader, header)
	
	// Read attrs block
	inAttrs := true
	attrs := strings.Builder{}
	body := strings.Builder{}
	
	for {
		line, err := reader.ReadString('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "Read error: %v\n", err)
			break
		}
		
		if inAttrs {
			if line == "\n" {
				inAttrs = false
				continue
			}
			attrs.WriteString(line)
		} else {
			body.WriteString(line)
		}
	}
	
	// Print response
	fmt.Print(attrs.String())
	if body.Len() > 0 {
		fmt.Print("\n")
		fmt.Print(body.String())
	}
}

func runInteractive(conn net.Conn) {
	scanner := bufio.NewScanner(os.Stdin)
	
	// Send header
	conn.Write([]byte("TXT01"))
	
	fmt.Println("Interactive mode. Type commands or 'exit' to quit.")
	fmt.Print("> ")
	
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		
		if line == "exit" || line == "quit" {
			break
		}
		
		if line == "" {
			fmt.Print("> ")
			continue
		}
		
		// Parse command
		parts := strings.Fields(line)
		if len(parts) == 0 {
			fmt.Print("> ")
			continue
		}
		
		cmd := parts[0]
		args := parts[1:]
		
		// Send command
		for _, arg := range args {
			if strings.HasPrefix(arg, `"`) {
				conn.Write([]byte(arg))
			} else {
				conn.Write([]byte(`"` + arg))
			}
			conn.Write([]byte{'\n'})
		}
		
		conn.Write([]byte(cmd))
		conn.Write([]byte{'\n'})
		
		// Read response
		readResponse(conn)
		
		fmt.Print("> ")
	}
	
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
	}
}

