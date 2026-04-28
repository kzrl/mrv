package tools

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

const maxSearchMatches = 50

// SearchCodeArgs are the input arguments for the SearchCode tool.
type SearchCodeArgs struct {
	Pattern   string `json:"pattern"             jsonschema_description:"Regular expression to search for"`
	Directory string `json:"directory,omitempty" jsonschema_description:"Directory to search in (default: current directory)"`
	Glob      string `json:"glob,omitempty"      jsonschema_description:"Filename glob to restrict search (e.g. *.go, *.ts). Matches against the filename only, not the full path."`
}

// SearchMatch is a single line that matched the pattern.
type SearchMatch struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Text string `json:"text"`
}

// SearchCodeResult is the output of the SearchCode tool.
type SearchCodeResult struct {
	Matches   []SearchMatch `json:"matches"`
	Truncated bool          `json:"truncated,omitempty"`
}

// NewSearchCodeTool creates a tool that searches for a regex pattern across files.
func NewSearchCodeTool() (tool.Tool, error) {
	return functiontool.New(functiontool.Config{
		Name:        "search_code",
		Description: "Search for a regular expression pattern across files in a directory. Returns file path, line number, and matching line for each match. Results are capped at 50.",
	}, searchCode)
}

func searchCode(_ tool.Context, args SearchCodeArgs) (SearchCodeResult, error) {
	if args.Pattern == "" {
		return SearchCodeResult{}, fmt.Errorf("search_code: pattern must not be empty")
	}

	re, err := regexp.Compile(args.Pattern)
	if err != nil {
		return SearchCodeResult{}, fmt.Errorf("search_code: invalid pattern: %w", err)
	}

	dir := args.Directory
	if dir == "" {
		dir = "."
	}

	var matches []SearchMatch
	truncated := false

	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if info.IsDir() {
			// Skip hidden directories (e.g. .git, .cache).
			if strings.HasPrefix(info.Name(), ".") && path != dir {
				return filepath.SkipDir
			}
			return nil
		}

		// Apply glob filter against the filename.
		if args.Glob != "" {
			matched, err := filepath.Match(args.Glob, info.Name())
			if err != nil {
				return fmt.Errorf("search_code: invalid glob: %w", err)
			}
			if !matched {
				return nil
			}
		}

		// Skip binary files.
		if isBinary(path) {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return nil // skip unreadable files
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			if re.MatchString(line) {
				if len(matches) >= maxSearchMatches {
					truncated = true
					return filepath.SkipAll
				}
				matches = append(matches, SearchMatch{
					File: path,
					Line: lineNum,
					Text: strings.TrimSpace(line),
				})
			}
		}
		return nil
	})
	if err != nil {
		return SearchCodeResult{}, fmt.Errorf("search_code: %w", err)
	}

	return SearchCodeResult{Matches: matches, Truncated: truncated}, nil
}

// isBinary reports whether the file looks binary by checking for null bytes
// in the first 512 bytes.
func isBinary(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	buf := make([]byte, 512)
	n, err := f.Read(buf)
	if err != nil {
		return false
	}
	return bytes.IndexByte(buf[:n], 0) != -1
}
