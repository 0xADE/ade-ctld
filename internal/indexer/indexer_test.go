package indexer

import (
	"context"
	"os"
	"path/filepath"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

var _ = ginkgo.Describe("Reindex", func() {
	var (
		idx   *Indexer
		ctx   context.Context
		paths []string
		count int
		err   error
		tmpDir string
	)

	ginkgo.BeforeEach(func() {
		idx = NewIndexer()
		ctx = context.Background()
	})

	ginkgo.AfterEach(func() {
		if tmpDir != "" {
			os.RemoveAll(tmpDir)
			tmpDir = ""
		}
	})

	ginkgo.Context("when reindexing with specific paths", func() {
		var binDir, appsDir string

		ginkgo.BeforeEach(func() {
			// Create temporary directory structure
			tmpDir, err = os.MkdirTemp("", "ade-ctld-test-*")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			// Create subdirectories
			binDir = filepath.Join(tmpDir, "bin")
			gomega.Expect(os.MkdirAll(binDir, 0755)).To(gomega.Succeed())

			appsDir = filepath.Join(tmpDir, "apps")
			gomega.Expect(os.MkdirAll(appsDir, 0755)).To(gomega.Succeed())

			// Create executable files
			exec1 := filepath.Join(binDir, "test1")
			gomega.Expect(os.WriteFile(exec1, []byte("#!/bin/sh\necho test1"), 0755)).To(gomega.Succeed())

			exec2 := filepath.Join(appsDir, "test2")
			gomega.Expect(os.WriteFile(exec2, []byte("#!/bin/sh\necho test2"), 0755)).To(gomega.Succeed())

			paths = []string{binDir, appsDir}
			count, err = idx.Reindex(ctx, paths)
		})

		ginkgo.It("should succeed", func() {
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should index at least 2 entries", func() {
			gomega.Expect(count).To(gomega.BeNumerically(">=", 2))
		})

		ginkgo.It("should add entries to the index", func() {
			index := idx.GetIndex()
			allEntries := index.GetAll()
			gomega.Expect(len(allEntries)).To(gomega.BeNumerically(">=", 2))
		})
	})

	ginkgo.Context("when reindexing without paths (nil)", func() {
		ginkgo.BeforeEach(func() {
			paths = nil
			count, err = idx.Reindex(ctx, paths)
		})

		ginkgo.It("should succeed", func() {
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should return non-negative count", func() {
			gomega.Expect(count).To(gomega.BeNumerically(">=", 0))
		})
	})

	ginkgo.Context("when reindexing with empty paths slice", func() {
		ginkgo.BeforeEach(func() {
			paths = []string{}
			count, err = idx.Reindex(ctx, paths)
		})

		ginkgo.It("should succeed", func() {
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should return non-negative count", func() {
			gomega.Expect(count).To(gomega.BeNumerically(">=", 0))
		})
	})
})

