package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func TestLoadReleaseNotes(t *testing.T) {
	t.Run("from inline string", func(t *testing.T) {
		got, err := loadReleaseNotes("My notes", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "My notes" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("from file overrides inline", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "notes.txt")
		_ = os.WriteFile(filePath, []byte("file content"), 0o644)

		got, err := loadReleaseNotes("inline content", filePath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "file content" {
			t.Errorf("got %q, want file content", got)
		}
	})

	t.Run("from file when inline is empty", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "notes.txt")
		_ = os.WriteFile(filePath, []byte("notes from file"), 0o644)

		got, err := loadReleaseNotes("", filePath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "notes from file" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("missing file returns error", func(t *testing.T) {
		_, err := loadReleaseNotes("", "/nonexistent/notes.txt")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("neither notes nor file returns error", func(t *testing.T) {
		_, err := loadReleaseNotes("", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestCommandBuilders(t *testing.T) {
	builders := []func() *cobra.Command{
		dumpVersionsCmd,
		addVersionCmd,
		dumpProductCmd,
		listProductsCmd,
		updateProductCmd,
		cloneProductCmd,
		releaseCmd,
	}
	for _, b := range builders {
		cmd := b()
		if cmd == nil {
			t.Errorf("%T returned nil", b)
			continue
		}
		if cmd.Use == "" {
			t.Errorf("command has empty Use field")
		}
	}
}
