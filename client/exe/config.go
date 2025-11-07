package exe

import (
	"fmt"
	"os"
	"os/user"
	"strings"
)

// getSocketPath returns the Unix socket path for ade-exe-ctld
func getSocketPath() (string, error) {
	// Check environment variable first
	socketPath := os.Getenv("ADE_INDEXD_SOCK")
	if socketPath != "" {
		// Expand tilde if present
		if strings.HasPrefix(socketPath, "~") {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("failed to get home directory: %w", err)
			}
			socketPath = strings.Replace(socketPath, "~", home, 1)
		}
		return socketPath, nil
	}

	// Default: use user ID-based path
	currentUser, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("failed to get current user: %w", err)
	}
	return fmt.Sprintf("/tmp/ade-%s/indexd", currentUser.Uid), nil
}
