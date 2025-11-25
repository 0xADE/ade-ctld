package parser

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ParseCommand", func() {
	var (
		input    string
		reader   *strings.Reader
		parser   *Parser
		cmd      *Command
		parseErr error
	)

	JustBeforeEach(func() {
		reader = strings.NewReader(input)
		parser, parseErr = NewParser(reader)
		Expect(parseErr).NotTo(HaveOccurred())

		cmd, parseErr = parser.ParseCommand()
		Expect(parseErr).NotTo(HaveOccurred())
	})

	Context("when parsing reindex command with arguments", func() {
		BeforeEach(func() {
			input = `TXT01
"~/bin
"~/apps
reindex
`
		})

		It("should parse command name correctly", func() {
			Expect(cmd.Name).To(Equal("reindex"))
		})

		It("should parse two arguments", func() {
			Expect(cmd.Args).To(HaveLen(2))
		})

		It("should parse first argument as string ~/bin", func() {
			Expect(cmd.Args[0].Type).To(Equal(TypeString))
			Expect(cmd.Args[0].Str).To(Equal("~/bin"))
		})

		It("should parse second argument as string ~/apps", func() {
			Expect(cmd.Args[1].Type).To(Equal(TypeString))
			Expect(cmd.Args[1].Str).To(Equal("~/apps"))
		})
	})

	Context("when parsing reindex command without arguments", func() {
		BeforeEach(func() {
			input = `TXT01
reindex
`
		})

		It("should parse command name correctly", func() {
			Expect(cmd.Name).To(Equal("reindex"))
		})

		It("should have no arguments", func() {
			Expect(cmd.Args).To(HaveLen(0))
		})
	})
})

