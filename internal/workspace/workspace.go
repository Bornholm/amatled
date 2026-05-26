package workspace

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

var (
	ErrOutsideWorkspace = errors.New("path is outside workspace")
	ErrNotDirectory     = errors.New("root path is not a directory")
)

// ignoredDirs are directories skipped during tree traversal.
var ignoredDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"dist":         true,
	".cache":       true,
}

// FileEntry représente un fichier ou dossier dans l'arborescence.
type FileEntry struct {
	Path     string      `json:"path"`
	Name     string      `json:"name"`
	IsDir    bool        `json:"isDir"`
	Children []FileEntry `json:"children,omitempty"`
}

// Workspace représente un dossier racine ouvert.
type Workspace struct {
	RootPath string
}

// Open ouvre un workspace à partir d'un chemin absolu.
func Open(rootPath string) (*Workspace, error) {
	info, err := os.Stat(rootPath)
	if err != nil {
		return nil, fmt.Errorf("stat root: %w", err)
	}
	if !info.IsDir() {
		return nil, ErrNotDirectory
	}
	abs, err := filepath.Abs(rootPath)
	if err != nil {
		return nil, fmt.Errorf("abs path: %w", err)
	}
	return &Workspace{RootPath: abs}, nil
}

// ListFiles retourne l'arborescence des fichiers .md du workspace.
func (w *Workspace) ListFiles() ([]FileEntry, error) {
	return listDir(w.RootPath, w.RootPath)
}

func listDir(rootPath, dirPath string) ([]FileEntry, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("read dir %s: %w", dirPath, err)
	}

	var result []FileEntry
	for _, e := range entries {
		name := e.Name()
		fullPath := filepath.Join(dirPath, name)

		if e.IsDir() {
			if ignoredDirs[name] || strings.HasPrefix(name, ".") {
				continue
			}
			children, err := listDir(rootPath, fullPath)
			if err != nil {
				continue
			}
			if len(children) == 0 {
				continue
			}
			relPath, _ := filepath.Rel(rootPath, fullPath)
			result = append(result, FileEntry{
				Path:     relPath,
				Name:     name,
				IsDir:    true,
				Children: children,
			})
		} else {
			if !strings.HasSuffix(strings.ToLower(name), ".md") {
				continue
			}
			relPath, _ := filepath.Rel(rootPath, fullPath)
			result = append(result, FileEntry{
				Path:  relPath,
				Name:  name,
				IsDir: false,
			})
		}
	}
	return result, nil
}

// AbsPath résout un chemin relatif au workspace et vérifie qu'il est bien dedans.
func (w *Workspace) AbsPath(relOrAbs string) (string, error) {
	return w.absPath(relOrAbs)
}

func (w *Workspace) absPath(relOrAbs string) (string, error) {
	p := relOrAbs
	if !filepath.IsAbs(p) {
		p = filepath.Join(w.RootPath, p)
	}
	p = filepath.Clean(p)
	rel, err := filepath.Rel(w.RootPath, p)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", ErrOutsideWorkspace
	}
	return p, nil
}

// ReadFile lit le contenu d'un fichier du workspace.
func (w *Workspace) ReadFile(path string) (string, error) {
	abs, err := w.absPath(path)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}
	return string(data), nil
}

// WriteFile écrit le contenu dans un fichier du workspace.
func (w *Workspace) WriteFile(path, content string) error {
	abs, err := w.absPath(path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(abs), fs.ModePerm); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	return os.WriteFile(abs, []byte(content), 0644)
}

// CreateFile crée un fichier vide dans le workspace.
func (w *Workspace) CreateFile(path string) error {
	return w.WriteFile(path, "")
}
