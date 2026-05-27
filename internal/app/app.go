package app

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"

	"github.com/bornholm/amatled/internal/settings"
	"github.com/bornholm/amatled/internal/updater"
	"github.com/zserge/lorca"
)

// App orchestre le lifecycle de l'application.
type App struct {
	bridge           *Bridge
	version          string
	initialWorkspace string
}

func New(version, initialWorkspace string) *App {
	s, err := settings.Load()
	if err != nil {
		slog.Warn("failed to load settings", "err", err)
		s = &settings.Settings{}
	}
	return &App{
		bridge:           newBridge(s, version),
		version:          version,
		initialWorkspace: initialWorkspace,
	}
}

func (a *App) Bridge() *Bridge {
	return a.bridge
}

// Run démarre le serveur HTTP local, ouvre la fenêtre lorca et bloque jusqu'à fermeture.
func (a *App) Run(webFS embed.FS) error {
	addr, err := startFileServer(webFS, a.bridge)
	if err != nil {
		return fmt.Errorf("start file server: %w", err)
	}

	url := "http://" + addr

	slog.Info("serving web assets", "url", url)

	ui, err := lorca.New(
		lorca.WithURL(url),
		lorca.WithWindowSize(1280, 800),
	)
	if err != nil {
		return fmt.Errorf("create lorca UI: %w", err)
	}
	defer ui.Close()

	if err := a.bridge.Register(ui); err != nil {
		return fmt.Errorf("register bridge: %w", err)
	}

	// Vérification de mise à jour au démarrage (non-bloquant).
	if a.bridge.settings.IsAutoUpdate() && a.version != "dev" {
		go func() {
			release, err := updater.Check(context.Background(), a.version)
			if err != nil {
				slog.Warn("startup update check failed", "err", err)
				return
			}
			if release != nil {
				a.bridge.SetPendingRelease(release)
				a.bridge.Emit("updater.updateAvailable", map[string]any{"version": release.Version()})
			}
		}()
	}

	// Ouvre le workspace initial (argument CLI) ou restaure le dernier workspace connu.
	workspaceToOpen := a.initialWorkspace
	if workspaceToOpen == "" {
		workspaceToOpen = a.bridge.settings.LastWorkspace
	}
	if workspaceToOpen != "" {
		go func() {
			result, err := a.bridge.handleOpenWorkspace(
				fmt.Sprintf(`{"path":%q}`, workspaceToOpen),
			)
			if err != nil {
				slog.Warn("failed to open workspace", "path", workspaceToOpen, "err", err)
				return
			}
			a.bridge.Emit("workspace.opened", result)
		}()
	}

	<-ui.Done()
	return nil
}

// startFileServer démarre un serveur HTTP qui sert les assets embarqués et l'API PDF.
func startFileServer(webFS embed.FS, bridge *Bridge) (string, error) {
	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		return "", fmt.Errorf("sub fs: %w", err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("listen: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/pdf", func(w http.ResponseWriter, r *http.Request) {
		fileId := r.URL.Query().Get("fileId")
		pdf, ok := bridge.GetCachedPDF(fileId)
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", "inline")
		w.Write(pdf)
	})
	mux.Handle("/", http.FileServer(http.FS(sub)))

	go func() {
		if err := http.Serve(listener, mux); err != nil {
			slog.Error("file server error", "err", err)
		}
	}()

	return listener.Addr().String(), nil
}
