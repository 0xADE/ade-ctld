package runindex

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.etcd.io/bbolt"
)

const (
	dbFile        = "exe-ctld.run-index"
	bucketName    = "run_index"
	dbPermissions = 0600
)

// RunIndex manages the run frequency index using bbolt DB.
type RunIndex struct {
	db *bbolt.DB
}

// NewRunIndex creates or opens the bbolt database for the run index.
func NewRunIndex() (*RunIndex, error) {
	// Get cache directory
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user cache directory: %w", err)
	}

	// Create ade directory in cache if it doesn't exist
	adeCacheDir := filepath.Join(cacheDir, "ade")
	if err := os.MkdirAll(adeCacheDir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	dbPath := filepath.Join(adeCacheDir, dbFile)

	// Open the bbolt database
	db, err := bbolt.Open(dbPath, dbPermissions, &bbolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Create the bucket if it doesn't exist
	err = db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(bucketName))
		if err != nil {
			return fmt.Errorf("failed to create bucket: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &RunIndex{db: db}, nil
}

// Increment increases the run count for a given path.
func (ri *RunIndex) Increment(path string) error {
	return ri.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		if b == nil {
			return fmt.Errorf("bucket %s not found", bucketName)
		}

		// Get current count
		val := b.Get([]byte(path))
		var count uint64
		if val != nil {
			count = binary.BigEndian.Uint64(val)
		}

		// Increment count
		count++

		// Put new count
		buf := make([]byte, 8)
		binary.BigEndian.PutUint64(buf, count)
		return b.Put([]byte(path), buf)
	})
}

// GetFrequencies retrieves the run frequencies for a list of paths.
func (ri *RunIndex) GetFrequencies(paths []string) map[string]uint64 {
	frequencies := make(map[string]uint64)
	ri.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		if b == nil {
			return nil // Bucket doesn't exist, no frequencies
		}

		for _, path := range paths {
			val := b.Get([]byte(path))
			if val != nil {
				frequencies[path] = binary.BigEndian.Uint64(val)
			} else {
				frequencies[path] = 0
			}
		}
		return nil
	})
	return frequencies
}

// Close closes the database connection.
func (ri *RunIndex) Close() error {
	if ri.db != nil {
		return ri.db.Close()
	}
	return nil
}
