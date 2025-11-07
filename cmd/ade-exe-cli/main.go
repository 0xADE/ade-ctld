package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/0xADE/ade-ctld/internal/config"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <command> [args...]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Commands:\n")
		fmt.Fprintf(os.Stderr, "  list                     - List all applications\n")
		fmt.Fprintf(os.Stderr, "  list-next <offset> [limit] - Get next page of results\n")
		fmt.Fprintf(os.Stderr, "  filter-name <name>       - Filter by name\n")
		fmt.Fprintf(os.Stderr, "  filter-cat <cat>         - Filter by category\n")
		fmt.Fprintf(os.Stderr, "  reset-filters            - Reset all filters\n")
		fmt.Fprintf(os.Stderr, "  run <id>                 - Run application by ID\n")
		fmt.Fprintf(os.Stderr, "  lang <locale>            - Set language\n")
		fmt.Fprintf(os.Stderr, "  interactive              - Interactive mode\n")
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
	log.Printf("[DEBUG] Connecting to socket: %s", socketPath)
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to socket %s: %v\n", socketPath, err)
		os.Exit(1)
	}
	defer conn.Close()
	log.Printf("[DEBUG] Connected successfully")

	cmd := os.Args[1]

	if cmd == "interactive" {
		runInteractive(conn)
		return
	}

	// Send header
	log.Printf("[DEBUG] Sending header TXT01")
	_, err = conn.Write([]byte("TXT01"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to send header: %v\n", err)
		os.Exit(1)
	}

	// Execute command
	switch cmd {
	case "list":
		sendCommand(conn, "list", nil)
	case "list-next":
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "Usage: %s list-next <offset> [limit_size]\n", os.Args[0])
			os.Exit(1)
		}
		sendCommand(conn, "list-next", os.Args[2:])
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

	// Close connection and exit in non-interactive mode
	log.Printf("[DEBUG] Closing connection and exiting")
	conn.Close()
	os.Exit(0)
}

// formatArgument formats an argument according to its type
// Returns the formatted string to send (without newline)
func formatArgument(arg string) string {
	arg = strings.TrimSpace(arg)

	// If starts with ", it's a string (keep prefix)
	if strings.HasPrefix(arg, `"`) {
		return arg
	}

	// Check for boolean literals
	if arg == "t" {
		return "t"
	}
	if arg == "f" {
		return "f"
	}

	// Check for recognized keywords (boolean operators)
	keywords := []string{"or", "and", "not"}
	for _, kw := range keywords {
		if arg == kw {
			return arg
		}
	}

	// Check if it's numeric (all digits)
	if _, err := strconv.ParseInt(arg, 10, 64); err == nil {
		return arg
	}

	// Default: treat as string (add prefix)
	return `"` + arg
}

func sendCommand(conn net.Conn, cmdName string, args []string) {
	log.Printf("[DEBUG] Sending command: %s with %d args", cmdName, len(args))

	// Send arguments with type detection
	for _, arg := range args {
		formatted := formatArgument(arg)
		fmt.Fprintf(conn, "%s", formatted)
		conn.Write([]byte{'\n'})
	}

	// Send command
	conn.Write([]byte(cmdName))
	conn.Write([]byte{'\n'})
	log.Printf("[DEBUG] Command sent")
}

func readResponse(conn net.Conn) {
	log.Printf("[DEBUG] Reading response...")
	reader := bufio.NewReader(conn)

	// Read header
	header := make([]byte, 5)
	n, err := io.ReadFull(reader, header)
	if err != nil {
		log.Printf("[ERROR] Failed to read header: %v (read %d bytes)", err, n)
		fmt.Fprintf(os.Stderr, "Failed to read response header: %v\n", err)
		return
	}
	log.Printf("[DEBUG] Header received: %s (%d bytes)", string(header), n)

	// Read attrs block and check for body: header
	attrs := strings.Builder{}
	body := strings.Builder{}
	hasBody := false
	seenBodyHeader := false

	for {
		line, err := reader.ReadString('\n')
		if err == io.EOF {
			log.Printf("[DEBUG] EOF reached")
			break
		}
		if err != nil {
			log.Printf("[ERROR] Read error: %v", err)
			fmt.Fprintf(os.Stderr, "Read error: %v\n", err)
			break
		}

		// Check if this is the body: header
		if strings.TrimSpace(line) == "body:" {
			seenBodyHeader = true
			hasBody = true
			log.Printf("[DEBUG] Found body: header")
			// Continue to read body content (don't add body: to attrs or body)
			continue
		}

		// Check for end of response marker (\n\n)
		if line == "\n" {
			peek, peekErr := reader.Peek(1)
			if peekErr == nil && len(peek) > 0 {
				if peek[0] == '\n' {
					// Skip the second \n
					reader.ReadByte()
					log.Printf("[DEBUG] End of response marker (\n\n) found")
					// Response ends here (with or without body)
					break
				}
				// Check if next line might be body: header
				// Peek ahead to see if next line is "body:"
				peekBytes, peekErr := reader.Peek(6) // "body:" + "\n" = 6 bytes
				if peekErr == nil && len(peekBytes) >= 5 {
					peekLine := string(peekBytes[:5])
					if peekLine == "body:" {
						// This blank line is part of headers, save it
						attrs.WriteString(line)
						continue
					}
				}
			}
			// Single \n in headers or body - save it
			if !seenBodyHeader {
				attrs.WriteString(line)
			} else {
				body.WriteString(line)
			}
			continue
		}

		if !seenBodyHeader {
			// Still reading headers
			attrs.WriteString(line)
		} else {
			// Reading body content
			body.WriteString(line)
		}
	}

	log.Printf("[DEBUG] Response received - attrs: %d bytes, body: %d bytes", attrs.Len(), body.Len())

	// Build full response for logging
	fullResponse := attrs.String()
	if hasBody {
		fullResponse += "body:\n" + body.String()
	}

	// Log response
	log.Printf("[RESPONSE] %s", fullResponse)

	// Print response to stdout
	fmt.Print(attrs.String())
	if hasBody {
		fmt.Print("body:\n")
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

		// Send command with type detection
		for _, arg := range args {
			formatted := formatArgument(arg)
			conn.Write([]byte(formatted))
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
