package runindex

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("RunIndex", func() {
	var (
		ri        *RunIndex
		testCacheDir string
	)

	BeforeEach(func() {
		// Create a temporary directory to use as cache directory
		var err error
		testCacheDir, err = os.MkdirTemp("", "ade-runindex-test-*")
		Expect(err).NotTo(HaveOccurred())

		ri, err = NewRunIndexWithCacheDir(testCacheDir)
		Expect(err).NotTo(HaveOccurred())
		Expect(ri).NotTo(BeNil())
	})

	AfterEach(func() {
		if ri != nil {
			err := ri.Close()
			Expect(err).NotTo(HaveOccurred())
		}

		// Clean up the temporary directory
		if testCacheDir != "" {
			err := os.RemoveAll(testCacheDir)
			Expect(err).NotTo(HaveOccurred())
		}
	})

	Describe("NewRunIndex", func() {
		It("should create a new RunIndex successfully", func() {
			Expect(ri).NotTo(BeNil())
			Expect(ri.db).NotTo(BeNil())
		})

		Context("when cache directory does not exist", func() {
			BeforeEach(func() {
				// The NewRunIndex function should create the ade directory if it doesn't exist
				// This is already tested in the main BeforeEach
			})

			It("should create the ade directory in cache", func() {
				adeCacheDir := filepath.Join(testCacheDir, "ade")
				Expect(adeCacheDir).To(BeADirectory())
			})

			It("should create the database file", func() {
				dbPath := filepath.Join(testCacheDir, "ade", "exe-ctld.run-index")
				Expect(dbPath).To(BeAnExistingFile())
			})
		})
	})

	Describe("Increment", func() {
		It("should increment the count for a path", func() {
			path := "/some/test/path"
			
			// Check initial count is 0
			freqs := ri.GetFrequencies([]string{path})
			Expect(freqs[path]).To(Equal(uint64(0)))

			// Increment and check
			err := ri.Increment(path)
			Expect(err).NotTo(HaveOccurred())

			freqs = ri.GetFrequencies([]string{path})
			Expect(freqs[path]).To(Equal(uint64(1)))

			// Increment again
			err = ri.Increment(path)
			Expect(err).NotTo(HaveOccurred())

			freqs = ri.GetFrequencies([]string{path})
			Expect(freqs[path]).To(Equal(uint64(2)))
		})

		It("should handle multiple different paths", func() {
			path1 := "/path/one"
			path2 := "/path/two"
			
			// Initial counts should be 0
			freqs := ri.GetFrequencies([]string{path1, path2})
			Expect(freqs[path1]).To(Equal(uint64(0)))
			Expect(freqs[path2]).To(Equal(uint64(0)))

			// Increment path1 twice and path2 once
			err := ri.Increment(path1)
			Expect(err).NotTo(HaveOccurred())
			err = ri.Increment(path1)
			Expect(err).NotTo(HaveOccurred())
			err = ri.Increment(path2)
			Expect(err).NotTo(HaveOccurred())

			freqs = ri.GetFrequencies([]string{path1, path2})
			Expect(freqs[path1]).To(Equal(uint64(2)))
			Expect(freqs[path2]).To(Equal(uint64(1)))
		})

		It("should handle increment errors", func() {
			// The bbolt database should be valid in our test setup, so this is more of a defensive test
			// For real error cases, we'd need to mock the bbolt behavior
			path := "/another/test/path"
			err := ri.Increment(path)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("GetFrequencies", func() {
		It("should return zero for paths that have not been incremented", func() {
			paths := []string{"/path/one", "/path/two", "/path/three"}
			
			freqs := ri.GetFrequencies(paths)
			for _, path := range paths {
				Expect(freqs[path]).To(Equal(uint64(0)))
			}
		})

		It("should return correct frequencies for paths that have been incremented", func() {
			// Set up some test data
			path1 := "/test/path/1"
			path2 := "/test/path/2"
			path3 := "/test/path/3"

			// Increment path1 3 times, path2 1 time, path3 5 times
			for i := 0; i < 3; i++ {
				err := ri.Increment(path1)
				Expect(err).NotTo(HaveOccurred())
			}
			err := ri.Increment(path2)
			Expect(err).NotTo(HaveOccurred())
			for i := 0; i < 5; i++ {
				err := ri.Increment(path3)
				Expect(err).NotTo(HaveOccurred())
			}

			// Test GetFrequencies
			paths := []string{path1, path2, path3, "/non/existent/path"}
			freqs := ri.GetFrequencies(paths)

			Expect(freqs[path1]).To(Equal(uint64(3)))
			Expect(freqs[path2]).To(Equal(uint64(1)))
			Expect(freqs[path3]).To(Equal(uint64(5)))
			Expect(freqs["/non/existent/path"]).To(Equal(uint64(0)))
		})

		It("should handle empty paths slice", func() {
			freqs := ri.GetFrequencies([]string{})
			Expect(freqs).To(BeEmpty())
		})

		It("should handle nil paths slice", func() {
			freqs := ri.GetFrequencies(nil)
			Expect(freqs).To(BeEmpty())
		})
	})

	Describe("Close", func() {
		It("should close the database successfully", func() {
			// Close the current instance
			err := ri.Close()
			Expect(err).NotTo(HaveOccurred())

			// Set to nil to prevent AfterEach from trying to close again
			ri = nil
		})

		It("should handle multiple close calls gracefully", func() {
			err := ri.Close()
			Expect(err).NotTo(HaveOccurred())

			// Try to close again
			err = ri.Close()
			Expect(err).NotTo(HaveOccurred())

			// Set to nil to prevent AfterEach from trying to close again
			ri = nil
		})

		It("should handle nil database gracefully", func() {
			// Create a RunIndex with nil db to test this case
			riWithNilDB := &RunIndex{db: nil}
			err := riWithNilDB.Close()
			Expect(err).NotTo(HaveOccurred())
		})
	})
})