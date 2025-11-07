package executable

import (
	"os"
	"path/filepath"
	"strings"
)

// ScanPaths scans executable files in the given paths
func ScanPaths(paths []string, resultChan chan<- *ExecutableInfo) error {
	defer close(resultChan)

	for _, path := range paths {
		if err := scanPath(path, resultChan); err != nil {
			// Continue scanning other paths even if one fails
			continue
		}
	}
	return nil
}

// ExecutableInfo contains information about an executable file
type ExecutableInfo struct {
	Name string // Executable name
	Path string // Full path to executable
}

func scanPath(rootPath string, resultChan chan<- *ExecutableInfo) error {
	return filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// Skip directories we can't access
			if info != nil && info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Check if file is executable
		if !isExecutable(info) {
			return nil
		}

		// Skip hidden files (starting with .)
		baseName := filepath.Base(path)
		if strings.HasPrefix(baseName, ".") {
			return nil
		}

		resultChan <- &ExecutableInfo{
			Name: baseName,
			Path: path,
		}

		return nil
	})
}

func isExecutable(info os.FileInfo) bool {
	// Check if file has execute permission for user, group, or others
	mode := info.Mode()
	return mode&0111 != 0
}
