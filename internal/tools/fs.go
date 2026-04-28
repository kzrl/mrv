package tools

import (
	"fmt"
	"os"
	"path/filepath"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

// ReadFileArgs are the input arguments for the ReadFile tool.
type ReadFileArgs struct {
	Path string `json:"path" jsonschema_description:"Absolute or relative path to the file to read"`
}

// ReadFileResult is the output of the ReadFile tool.
type ReadFileResult struct {
	Content string `json:"content"`
}

// WriteFileArgs are the input arguments for the WriteFile tool.
type WriteFileArgs struct {
	Path    string `json:"path"    jsonschema_description:"Absolute or relative path to the file to write"`
	Content string `json:"content" jsonschema_description:"Content to write to the file"`
}

// WriteFileResult is the output of the WriteFile tool.
type WriteFileResult struct {
	OK bool `json:"ok"`
}

// ListFilesArgs are the input arguments for the ListFiles tool.
type ListFilesArgs struct {
	Directory string `json:"directory"           jsonschema_description:"Directory to list"`
	Recursive bool   `json:"recursive,omitempty" jsonschema_description:"Whether to list files recursively"`
}

// ListFilesResult is the output of the ListFiles tool.
type ListFilesResult struct {
	Files []string `json:"files"`
}

// NewReadFileTool creates a tool that reads a file from disk.
func NewReadFileTool() (tool.Tool, error) {
	return functiontool.New(functiontool.Config{
		Name:        "read_file",
		Description: "Read the contents of a file from the filesystem.",
	}, readFile)
}

// NewWriteFileTool creates a tool that writes content to a file.
// requireConfirmation controls whether the agent must ask before writing.
func NewWriteFileTool(requireConfirmation bool) (tool.Tool, error) {
	return functiontool.New(functiontool.Config{
		Name:                "write_file",
		Description:         "Write (overwrite) content to a file. Creates parent directories if needed.",
		RequireConfirmation: requireConfirmation,
	}, writeFile)
}

// NewListFilesTool creates a tool that lists files in a directory.
func NewListFilesTool() (tool.Tool, error) {
	return functiontool.New(functiontool.Config{
		Name:        "list_files",
		Description: "List files in a directory. Set recursive=true to walk subdirectories.",
	}, listFiles)
}

func readFile(_ tool.Context, args ReadFileArgs) (ReadFileResult, error) {
	data, err := os.ReadFile(args.Path)
	if err != nil {
		return ReadFileResult{}, fmt.Errorf("read_file: %w", err)
	}
	return ReadFileResult{Content: string(data)}, nil
}

func writeFile(_ tool.Context, args WriteFileArgs) (WriteFileResult, error) {
	if err := os.MkdirAll(filepath.Dir(args.Path), 0o755); err != nil {
		return WriteFileResult{}, fmt.Errorf("write_file: mkdir: %w", err)
	}
	if err := os.WriteFile(args.Path, []byte(args.Content), 0o644); err != nil {
		return WriteFileResult{}, fmt.Errorf("write_file: %w", err)
	}
	return WriteFileResult{OK: true}, nil
}

func listFiles(_ tool.Context, args ListFilesArgs) (ListFilesResult, error) {
	dir := args.Directory
	if dir == "" {
		dir = "."
	}
	var files []string
	if args.Recursive {
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return ListFilesResult{}, fmt.Errorf("list_files: %w", err)
		}
	} else {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return ListFilesResult{}, fmt.Errorf("list_files: %w", err)
		}
		for _, e := range entries {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	return ListFilesResult{Files: files}, nil
}
