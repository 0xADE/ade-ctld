package exe

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
)

// Application represents an application entry
type Application struct {
	ID   int64
	Name string
}

// Client handles connection to ade-exe-ctld server
type Client struct {
	conn   net.Conn
	mu     sync.Mutex
	socket string
}

const protoVer = "TXT01" // cmdlist protocol, text format, v01

// NewClient creates a new client and connects to the server
func NewClient() (*Client, error) {
	socketPath, err := getSocketPath()
	if err != nil {
		return nil, fmt.Errorf("failed to get socket path: %w", err)
	}

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to socket %s: %w", socketPath, err)
	}

	// Send header
	if _, err := conn.Write([]byte(protoVer)); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to send header: %w", err)
	}

	return &Client{
		conn:   conn,
		socket: socketPath,
	}, nil
}

// Close closes the connection
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// FormatArgument formats an argument according to its type
func FormatArgument(arg string) string {
	arg = strings.TrimSpace(arg)

	// If starts with ", it's a string (keep prefix)
	if strings.HasPrefix(arg, `"`) {
		return arg
	}

	// Check for boolean literals
	if arg == "t" || arg == "f" {
		return arg
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

// SendCommand sends a command to the server
func (c *Client) SendCommand(cmdName string, args []string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Send arguments with type detection
	for _, arg := range args {
		formatted := FormatArgument(arg)
		if _, err := fmt.Fprintf(c.conn, "%s\n", formatted); err != nil {
			return fmt.Errorf("failed to send argument: %w", err)
		}
	}

	// Send command
	if _, err := fmt.Fprintf(c.conn, "%s\n", cmdName); err != nil {
		return fmt.Errorf("failed to send command: %w", err)
	}

	return nil
}

// Conn returns the underlying connection
func (c *Client) Conn() net.Conn {
	return c.conn
}

func ReadResponse(conn net.Conn) {
	reader := bufio.NewReader(conn)

	// Read header
	header := make([]byte, 5)
	_, err := io.ReadFull(reader, header)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read response header: %v\n", err)
		return
	}

	// Read attrs block and check for body: header
	attrs := strings.Builder{}
	body := strings.Builder{}
	hasBody := false
	seenBodyHeader := false

	for {
		line, err := reader.ReadString('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "Read error: %v\n", err)
			break
		}

		// Check if this is the body: header
		if strings.TrimSpace(line) == "body:" {
			seenBodyHeader = true
			// Continue to read body content (don't add body: to attrs or body)
			continue
		}

		// Check for end of response marker (\n\n)
		if line == "\n" {
			if isEndOfResponse(reader) {
				// Response ends here (with or without body)
				break
			}

			// Check if this blank line is part of headers before body:
			if isBlankLineBeforeBodyHeader(reader) {
				// This blank line is part of headers, save it
				attrs.WriteString(line)
				continue
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

	// Build full response for logging
	fullResponse := attrs.String()
	if hasBody {
		fullResponse += "body:\n" + body.String()
	}

	// Print response to stdout
	fmt.Print(attrs.String())
	if hasBody {
		fmt.Print("body:\n")
		fmt.Print(body.String())
	}
}

// isEndOfResponse checks if we've reached the end of response marker (\n\n)
func isEndOfResponse(reader *bufio.Reader) bool {
	peek, peekErr := reader.Peek(1)
	if peekErr == nil && len(peek) > 0 && peek[0] == '\n' {
		// Skip the second \n
		reader.ReadByte()
		return true
	}
	return false
}

// isBlankLineBeforeBodyHeader checks if this blank line is followed by a "body:" header
func isBlankLineBeforeBodyHeader(reader *bufio.Reader) bool {
	peekBytes, peekErr := reader.Peek(6) // "body:" + "\n" = 6 bytes
	if peekErr == nil && len(peekBytes) >= 5 {
		peekLine := string(peekBytes[:5])
		if peekLine == "body:" {
			return true
		}
	}
	return false
}

// ResetFilters resets all filters
func (c *Client) ResetFilters() error {
	return c.SendCommand("0filters", nil)
}

// SetFilterName sets a name filter
func (c *Client) SetFilterName(query string) error {
	if query == "" {
		return c.ResetFilters()
	}
	return c.SendCommand("filter-name", []string{query})
}

// List retrieves the list of applications matching current filters
func (c *Client) List() ([]Application, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Send list command
	if _, err := fmt.Fprintf(c.conn, "list\n"); err != nil {
		return nil, fmt.Errorf("failed to send list command: %w", err)
	}

	// Read response
	attrs, body, err := c.readResponse()
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for errors
	if errMsg, ok := attrs["error"]; ok {
		return nil, fmt.Errorf("server error: %s", errMsg)
	}

	// Parse body
	var apps []Application
	lines := strings.Split(strings.TrimSpace(body), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		id, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			continue
		}
		name := strings.Join(parts[1:], " ")
		apps = append(apps, Application{
			ID:   id,
			Name: name,
		})
	}

	return apps, nil
}

// Run executes an application by ID
func (c *Client) Run(id int64) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Send run command with id
	if _, err := fmt.Fprintf(c.conn, "%d\nrun\n", id); err != nil {
		return fmt.Errorf("failed to send run command: %w", err)
	}

	// Read response
	attrs, _, err := c.readResponse()
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Check for errors
	if errMsg, ok := attrs["error"]; ok {
		return fmt.Errorf("server error: %s", errMsg)
	}

	return nil
}

// RunInTerminal executes an application by ID in a terminal
func (c *Client) RunInTerminal(id int64) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Send opt: terminal, id, run
	if _, err := fmt.Fprintf(c.conn, "\"opt: terminal\n%d\nrun\n", id); err != nil {
		return fmt.Errorf("failed to send run command: %w", err)
	}

	// Read response
	attrs, _, err := c.readResponse()
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Check for errors
	if errMsg, ok := attrs["error"]; ok {
		return fmt.Errorf("server error: %s", errMsg)
	}

	return nil
}

// readResponse is a private method that returns parsed response
func (c *Client) readResponse() (map[string]string, string, error) {
	reader := bufio.NewReader(c.conn)

	// Read header
	header := make([]byte, 5)
	_, err := io.ReadFull(reader, header)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read response header: %w", err)
	}

	// Read attrs block and check for body: header
	attrs := make(map[string]string)
	body := strings.Builder{}
	seenBodyHeader := false

	for {
		line, err := reader.ReadString('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, "", fmt.Errorf("read error: %w", err)
		}

		// Check if this is the body: header
		if strings.TrimSpace(line) == "body:" {
			seenBodyHeader = true
			// Continue to read body content (don't add body: to attrs or body)
			continue
		}

		// Check for end of response marker (\n\n)
		if line == "\n" {
			if isEndOfResponse(reader) {
				// Response ends here (with or without body)
				break
			}

			// Check if this blank line is part of headers before body:
			if isBlankLineBeforeBodyHeader(reader) {
				// This blank line is part of headers, save it
				continue
			}

			// Single \n in headers or body - save it
			if !seenBodyHeader {
				// In headers, blank lines are ignored
			} else {
				body.WriteString(line)
			}
			continue
		}

		if !seenBodyHeader {
			// Still reading headers
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				attrs[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
		} else {
			// Reading body content
			body.WriteString(line)
		}
	}

	return attrs, body.String(), nil
}
