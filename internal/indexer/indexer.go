package indexer

import (
	"context"
	"sync"

	"github.com/0xADE/ade-ctld/internal/config"
	"github.com/0xADE/ade-ctld/internal/indexer/desktop"
	"github.com/0xADE/ade-ctld/internal/indexer/executable"
)

// Indexer coordinates indexing of executables and desktop files
type Indexer struct {
	index   *Index
	running bool
	mu      sync.RWMutex
}

// NewIndexer creates a new indexer instance
func NewIndexer() *Indexer {
	return &Indexer{
		index: NewIndex(),
	}
}

// Start begins the indexing process
func (idx *Indexer) Start(ctx context.Context) error {
	idx.mu.Lock()
	if idx.running {
		idx.mu.Unlock()
		return nil
	}
	idx.running = true
	idx.mu.Unlock()
	
	// Clear existing index
	idx.index = NewIndex()
	
	// Create channels for results
	execChan := make(chan *executable.ExecutableInfo, 100)
	desktopChan := make(chan *desktop.DesktopEntry, 100)
	
	var wg sync.WaitGroup
	
	// Start executable scanning
	wg.Add(1)
	go func() {
		defer wg.Done()
		cfg := config.Get()
		paths := cfg.Path()
		if err := executable.ScanPaths(paths, execChan); err != nil {
			// Log error but continue
			return
		}
	}()
	
	// Start desktop file scanning
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := desktop.ScanDesktopFiles(desktopChan); err != nil {
			// Log error but continue
			return
		}
	}()
	
	// Process results
	wg.Add(1)
	go func() {
		defer wg.Done()
		idx.processResults(ctx, execChan, desktopChan)
	}()
	
	// Channels are closed by ScanPaths and ScanDesktopFiles when they finish
	// No need to close them here
	
	return nil
}

func (idx *Indexer) processResults(ctx context.Context, execChan <-chan *executable.ExecutableInfo, desktopChan <-chan *desktop.DesktopEntry) {
	for {
		select {
		case <-ctx.Done():
			return
		case exec, ok := <-execChan:
			if !ok {
				execChan = nil
			} else {
				entry := &Entry{
					Name:     exec.Name,
					Path:     exec.Path,
					Exec:     exec.Path,
					Terminal: false,
					IsDesktop: false,
				}
				idx.index.Add(entry)
			}
		case desk, ok := <-desktopChan:
			if !ok {
				desktopChan = nil
			} else {
				// Skip NoDisplay entries
				if desktop.IsNoDisplay(desk.Path) {
					continue
				}
				
				entry := &Entry{
					Name:       desk.Name,
					Names:      desk.Names,
					Path:       desk.Path,
					Exec:       desk.Exec,
					Terminal:   desk.Terminal,
					Categories: desk.Categories,
					IsDesktop:  true,
				}
				idx.index.Add(entry)
			}
		}
		
		if execChan == nil && desktopChan == nil {
			break
		}
	}
}

// GetIndex returns the index instance
func (idx *Indexer) GetIndex() *Index {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.index
}

// IsRunning returns whether indexing is currently running
func (idx *Indexer) IsRunning() bool {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.running
}

// Stop stops the indexing process
func (idx *Indexer) Stop() {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.running = false
}

