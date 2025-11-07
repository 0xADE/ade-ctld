package exe

import (
	"bufio"
	"fmt"
	"io"
	"net"
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

// formatArgument formats an argument according to its type
func formatArgument(arg string) string {
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

// sendCommand sends a command to the server
func (c *Client) sendCommand(cmdName string, args []string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Send arguments with type detection
	for _, arg := range args {
		formatted := formatArgument(arg)
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

// readResponse reads a response from the server
func (c *Client) readResponse() (map[string]string, string, error) {
	reader := bufio.NewReader(c.conn)

	// Read attrs block and check for body: header
	attrs := make(map[string]string)
	var body strings.Builder
	hasBody := false
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
			hasBody = true
			continue
		}

		// Check for end of response marker (\n\n)
		if line == "\n" {
			peek, peekErr := reader.Peek(1)
			if peekErr == nil && len(peek) > 0 {
				if peek[0] == '\n' {
					// Skip the second \n
					reader.ReadByte()
					break
				}
				// Check if next line might be body: header
				peekBytes, peekErr := reader.Peek(6)
				if peekErr == nil && len(peekBytes) >= 5 {
					peekLine := string(peekBytes[:5])
					if peekLine == "body:" {
						// This blank line is part of headers, continue
						continue
					}
				}
			}
		}

		if !seenBodyHeader {
			// Parse header line: "key: value"
			line = strings.TrimSpace(line)
			if line != "" {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					key := strings.TrimSpace(parts[0])
					value := strings.TrimSpace(parts[1])
					attrs[key] = value
				}
			}
		} else {
			// Reading body content
			body.WriteString(line)
		}
	}

	bodyStr := ""
	if hasBody {
		bodyStr = body.String()
	}

	return attrs, bodyStr, nil
}

// ResetFilters resets all filters
func (c *Client) ResetFilters() error {
	return c.sendCommand("0filters", nil)
}

// SetFilterName sets a name filter
func (c *Client) SetFilterName(query string) error {
	if query == "" {
		return c.ResetFilters()
	}
	return c.sendCommand("filter-name", []string{query})
}

// List gets the list of applications matching current filters
func (c *Client) List() ([]Application, error) {
	if err := c.sendCommand("list", nil); err != nil {
		return nil, err
	}

	attrs, body, err := c.readResponse()
	if err != nil {
		return nil, err
	}

	// Check for errors
	if status, ok := attrs["status"]; ok && status != "0" {
		return nil, fmt.Errorf("server error: %s", attrs["error"])
	}

	// Parse body
	applications := []Application{}
	lines := strings.SplitSeq(strings.TrimSpace(body), "\n")
	for line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse line: "ID Name"
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		id, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			continue
		}

		name := strings.Join(parts[1:], " ")
		applications = append(applications, Application{
			ID:   id,
			Name: name,
		})
	}

	return applications, nil
}

// Run runs an application by ID
func (c *Client) Run(id int64) error {
	idStr := strconv.FormatInt(id, 10)
	if err := c.sendCommand("run", []string{idStr}); err != nil {
		return err
	}

	attrs, _, err := c.readResponse()
	if err != nil {
		return err
	}

	// Check for errors
	if status, ok := attrs["status"]; ok && status != "0" {
		errorMsg := attrs["error"]
		if errorMsg == "" {
			errorMsg = attrs["desc"]
		}
		return fmt.Errorf("server error: %s", errorMsg)
	}

	return nil
}
