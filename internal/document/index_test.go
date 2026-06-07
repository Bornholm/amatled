package document_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bornholm/amatled/internal/document"
	"github.com/bornholm/amatled/internal/workspace"
)

func TestBuildWorkspaceIndexTruncatesOnFileCount(t *testing.T) {
	root := t.TempDir()
	for i := range 250 {
		content := fmt.Sprintf("# Fichier %d\n\nContenu.\n", i)
		path := filepath.Join(root, fmt.Sprintf("doc-%03d.md", i))
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}
	}

	ws, err := workspace.Open(root)
	if err != nil {
		t.Fatalf("open workspace: %v", err)
	}

	idx, err := document.BuildWorkspaceIndex(ws)
	if err != nil {
		t.Fatalf("build index: %v", err)
	}

	if !idx.Truncated {
		t.Errorf("expected Truncated=true for a workspace with 250 files")
	}
	if len(idx.Files) > 200 {
		t.Errorf("expected at most 200 indexed files, got %d", len(idx.Files))
	}

	formatted := document.FormatWorkspaceIndex(idx)
	if !strings.Contains(formatted, "index tronqué") {
		t.Errorf("expected formatted index to mention truncation, got: %s", formatted)
	}
}

func TestBuildWorkspaceIndexNotTruncatedForSmallWorkspace(t *testing.T) {
	root := t.TempDir()
	for i := range 3 {
		content := fmt.Sprintf("# Fichier %d\n\n## Section\n\nContenu.\n", i)
		path := filepath.Join(root, fmt.Sprintf("doc-%d.md", i))
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}
	}

	ws, err := workspace.Open(root)
	if err != nil {
		t.Fatalf("open workspace: %v", err)
	}

	idx, err := document.BuildWorkspaceIndex(ws)
	if err != nil {
		t.Fatalf("build index: %v", err)
	}

	if idx.Truncated {
		t.Errorf("expected Truncated=false for a small workspace")
	}
	if len(idx.Files) != 3 {
		t.Errorf("expected 3 indexed files, got %d", len(idx.Files))
	}

	formatted := document.FormatWorkspaceIndex(idx)
	if strings.Contains(formatted, "index tronqué") {
		t.Errorf("did not expect truncation mention for a small workspace, got: %s", formatted)
	}
}
