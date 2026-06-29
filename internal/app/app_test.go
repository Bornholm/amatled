package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveInitialWorkspace(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "doc.md")
	if err := os.WriteFile(testFile, []byte("# hello"), 0644); err != nil {
		t.Fatalf("create test file: %v", err)
	}

	t.Run("empty path", func(t *testing.T) {
		ws, file := resolveInitialWorkspace("")
		if ws != "" || file != "" {
			t.Fatalf("expected empty workspace and file, got %q, %q", ws, file)
		}
	})

	t.Run("directory path", func(t *testing.T) {
		ws, file := resolveInitialWorkspace(tmpDir)
		if ws != tmpDir || file != "" {
			t.Fatalf("expected workspace %q and empty file, got %q, %q", tmpDir, ws, file)
		}
	})

	t.Run("file path", func(t *testing.T) {
		ws, file := resolveInitialWorkspace(testFile)
		if ws != tmpDir {
			t.Fatalf("expected workspace %q, got %q", tmpDir, ws)
		}
		if file != "doc.md" {
			t.Fatalf("expected file doc.md, got %q", file)
		}
	})

	t.Run("nonexistent path", func(t *testing.T) {
		missing := filepath.Join(tmpDir, "missing")
		ws, file := resolveInitialWorkspace(missing)
		if ws != missing || file != "" {
			t.Fatalf("expected workspace %q and empty file, got %q, %q", missing, ws, file)
		}
	})
}
