package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEditFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")

	write := func(s string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	read := func() string {
		t.Helper()
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}

	t.Run("basic replacement", func(t *testing.T) {
		write("hello world")
		res, err := editFile(nil, EditFileArgs{Path: path, OldString: "world", NewString: "Go"})
		if err != nil {
			t.Fatal(err)
		}
		if !res.OK {
			t.Error("expected OK")
		}
		if got := read(); got != "hello Go" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("not found", func(t *testing.T) {
		write("hello world")
		_, err := editFile(nil, EditFileArgs{Path: path, OldString: "missing", NewString: "x"})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ambiguous", func(t *testing.T) {
		write("foo foo")
		_, err := editFile(nil, EditFileArgs{Path: path, OldString: "foo", NewString: "bar"})
		if err == nil {
			t.Fatal("expected error for ambiguous match")
		}
	})

	t.Run("replaces only first occurrence when unique context given", func(t *testing.T) {
		write("func A() {}\nfunc B() {}")
		res, err := editFile(nil, EditFileArgs{Path: path, OldString: "func A() {}", NewString: "func A() { return }"})
		if err != nil {
			t.Fatal(err)
		}
		if !res.OK {
			t.Error("expected OK")
		}
		if got := read(); got != "func A() { return }\nfunc B() {}" {
			t.Errorf("got %q", got)
		}
	})
}
