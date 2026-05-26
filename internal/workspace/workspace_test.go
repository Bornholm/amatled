package workspace_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bornholm/amatled/internal/workspace"
)

func TestListFiles(t *testing.T) {
	root := t.TempDir()
	// Créer une arborescence de test
	must(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("# Hello"), 0644))
	must(t, os.MkdirAll(filepath.Join(root, "docs"), 0755))
	must(t, os.WriteFile(filepath.Join(root, "docs", "guide.md"), []byte("# Guide"), 0644))
	must(t, os.WriteFile(filepath.Join(root, "docs", "image.png"), []byte("PNG"), 0644))
	must(t, os.MkdirAll(filepath.Join(root, ".git"), 0755))
	must(t, os.WriteFile(filepath.Join(root, ".git", "config"), []byte(""), 0644))
	must(t, os.MkdirAll(filepath.Join(root, "node_modules", "pkg"), 0755))
	must(t, os.WriteFile(filepath.Join(root, "node_modules", "pkg", "index.md"), []byte(""), 0644))

	ws, err := workspace.Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	files, err := ws.ListFiles()
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}

	paths := collectPaths(files)

	assertContains(t, paths, "README.md")
	assertContains(t, paths, filepath.Join("docs", "guide.md"))
	assertNotContains(t, paths, filepath.Join("docs", "image.png"))
	assertNotContains(t, paths, filepath.Join(".git", "config"))
	assertNotContains(t, paths, filepath.Join("node_modules", "pkg", "index.md"))
}

func TestReadWriteFile(t *testing.T) {
	root := t.TempDir()
	ws, _ := workspace.Open(root)

	if err := ws.WriteFile("test.md", "# Test"); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	content, err := ws.ReadFile("test.md")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if content != "# Test" {
		t.Errorf("expected '# Test', got %q", content)
	}
}

func TestReadFileOutsideWorkspace(t *testing.T) {
	root := t.TempDir()
	ws, _ := workspace.Open(root)

	_, err := ws.ReadFile("../../etc/passwd")
	if err != workspace.ErrOutsideWorkspace {
		t.Errorf("expected ErrOutsideWorkspace, got %v", err)
	}
}

func collectPaths(entries []workspace.FileEntry) []string {
	var paths []string
	for _, e := range entries {
		if e.IsDir {
			paths = append(paths, collectPaths(e.Children)...)
		} else {
			paths = append(paths, e.Path)
		}
	}
	return paths
}

func assertContains(t *testing.T, paths []string, want string) {
	t.Helper()
	for _, p := range paths {
		if p == want {
			return
		}
	}
	t.Errorf("expected path %q in %v", want, paths)
}

func assertNotContains(t *testing.T, paths []string, unwanted string) {
	t.Helper()
	for _, p := range paths {
		if p == unwanted {
			t.Errorf("unexpected path %q in results", unwanted)
			return
		}
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
