package parser

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// ValueType represents the type of a value on the stack
type ValueType int

const (
	TypeString ValueType = iota
	TypeInt
	TypeBool
)

// Value represents a value on the stack
type Value struct {
	Type ValueType
	Str  string
	Int  int64
	Bool bool
}

// Command represents a parsed command
type Command struct {
	Name string
	Args []Value
}

// Parser parses Forth-style commands
type Parser struct {
	reader  *bufio.Reader
	header  string
	version string
}

// NewParser creates a new parser
func NewParser(reader io.Reader) (*Parser, error) {
	p := &Parser{
		reader: bufio.NewReader(reader),
	}

	// Read header
	headerBytes := make([]byte, 5)
	if n, err := io.ReadFull(p.reader, headerBytes); err != nil || n != 5 {
		return nil, fmt.Errorf("invalid header")
	}

	p.header = string(headerBytes[:3])
	p.version = string(headerBytes[3:5])

	if p.header != "TXT" {
		return nil, fmt.Errorf("unsupported format: %s", p.header)
	}

	return p, nil
}

// ParseCommand parses the next command from input
func (p *Parser) ParseCommand() (*Command, error) {
	stack := make([]Value, 0)

	for {
		line, err := p.reader.ReadString('\n')
		if err == io.EOF {
			if len(stack) == 0 {
				return nil, io.EOF
			}
			// Return command if stack is not empty
			break
		}
		if err != nil {
			return nil, err
		}

		line = strings.TrimSpace(line)

		// Skip empty lines
		if line == "" {
			continue
		}

		// Skip comments
		if strings.HasPrefix(line, "#") {
			continue
		}

		// Check if it's a command
		if cmd := parseCommand(line); cmd != "" {
			// Return command with current stack
			return &Command{
				Name: cmd,
				Args: stack,
			}, nil
		}

		// Otherwise, parse as value and push to stack
		value, err := parseValue(line)
		if err != nil {
			return nil, fmt.Errorf("parse error: %v", err)
		}
		stack = append(stack, value)
	}

	return nil, io.EOF
}

func parseCommand(line string) string {
	line = strings.TrimSpace(line)

	// Known commands
	// reindex accepts arbitrary number of string path arguments
	commands := []string{
		"filter-name",
		"+filter-name",
		"+filter-cat",
		"+filter-path",
		"0filters",
		"list",
		"run",
		"lang",
		"saveconf",
		"list-next",
		"reindex",
	}

	for _, cmd := range commands {
		if line == cmd {
			return cmd
		}
	}

	return ""
}

func parseValue(line string) (Value, error) {
	line = strings.TrimSpace(line)

	// String value (prefixed with ")
	// Supports special option strings like "opt: terminal" for run command
	if after, ok := strings.CutPrefix(line, `"`); ok {
		str := after
		return Value{Type: TypeString, Str: str}, nil
	}

	// Boolean literals (t/f)
	switch line {
	case "t":
		return Value{Type: TypeBool, Bool: true}, nil
	case "f":
		return Value{Type: TypeBool, Bool: false}, nil
	}

	// Boolean operators (keywords)
	switch line {
	case "or":
		return Value{Type: TypeBool, Bool: true, Str: "or"}, nil // true = OR operation
	case "and":
		return Value{Type: TypeBool, Bool: false, Str: "and"}, nil // false = AND operation
	case "not":
		return Value{Type: TypeBool, Bool: false, Str: "not"}, nil // NOT operation
	}

	// Try parsing as integer (must be all digits)
	if intVal, err := strconv.ParseInt(line, 10, 64); err == nil {
		return Value{Type: TypeInt, Int: intVal}, nil
	}

	return Value{}, fmt.Errorf("cannot parse value: %s", line)
}

// ParseBoolOp parses boolean operation from value
func ParseBoolOp(v Value) string {
	if v.Type != TypeBool {
		return ""
	}
	// This is a simplified approach - in real implementation,
	// we might need to track operation type differently
	// For now, assume we track it through context
	return "or" // placeholder
}

// ReadAllCommands reads all commands from the parser
func (p *Parser) ReadAllCommands() ([]*Command, error) {
	var commands []*Command

	for {
		cmd, err := p.ParseCommand()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		commands = append(commands, cmd)
	}

	return commands, nil
}
