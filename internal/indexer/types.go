package indexer

import (
	"sync"
)

// Entry represents a single indexed application entry
type Entry struct {
	ID         int64             // Unique identifier
	Name       string            // Default name (English or fallback)
	Names      map[string]string // Localized names (locale -> name)
	Path       string            // Path to executable or .desktop file
	Exec       string            // Command to execute
	Terminal   bool              // Whether to run in terminal
	Categories []string          // Application categories
	IsDesktop  bool              // Whether this is from a .desktop file
}

// Index stores all indexed entries with thread-safe access
type Index struct {
	mu      sync.RWMutex
	entries map[int64]*Entry
	nextID  int64
}

// NewIndex creates a new empty index
func NewIndex() *Index {
	return &Index{
		entries: make(map[int64]*Entry),
		nextID:  1,
	}
}

// Add adds a new entry to the index and returns its ID
func (idx *Index) Add(entry *Entry) int64 {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	entry.ID = idx.nextID
	idx.nextID++
	idx.entries[entry.ID] = entry
	return entry.ID
}

// Get retrieves an entry by ID
func (idx *Index) Get(id int64) (*Entry, bool) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	entry, ok := idx.entries[id]
	return entry, ok
}

// GetAll returns all entries (for filtering)
func (idx *Index) GetAll() []*Entry {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	result := make([]*Entry, 0, len(idx.entries))
	for _, entry := range idx.entries {
		result = append(result, entry)
	}
	return result
}

// Count returns the number of entries in the index
func (idx *Index) Count() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return len(idx.entries)
}
