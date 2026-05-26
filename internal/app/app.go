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
	bridge  *Bridge
	version string
}

func New(version string) *App {
	s, err := settings.Load()
	if err != nil {
		slog.Warn("failed to load settings", "err", err)
		s = &settings.Settings{}
	}
	return &App{
		bridge:  newBridge(s, version),
		version: version,
	}
}

func (a *App) Bridge() *Bridge {
	return a.bridge
}

// Run démarre le serveur HTTP local, ouvre la fenêtre lorca et bloque jusqu'à fermeture.
func (a *App) Run(webFS embed.FS) error {
	addr, err := startFileServer(webFS)
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

	// Si un workspace était ouvert lors de la dernière session, le rouvrir.
	if a.bridge.settings.LastWorkspace != "" {
		go func() {
			result, err := a.bridge.handleOpenWorkspace(
				fmt.Sprintf(`{"path":%q}`, a.bridge.settings.LastWorkspace),
			)
			if err != nil {
				slog.Warn("failed to restore last workspace", "err", err)
				return
			}
			a.bridge.Emit("workspace.opened", result)
		}()
	}

	<-ui.Done()
	return nil
}

// startFileServer démarre un serveur HTTP qui sert les assets embarqués.
func startFileServer(webFS embed.FS) (string, error) {
	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		return "", fmt.Errorf("sub fs: %w", err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("listen: %w", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(sub)))

	go func() {
		if err := http.Serve(listener, mux); err != nil {
			slog.Error("file server error", "err", err)
		}
	}()

	return listener.Addr().String(), nil
}
