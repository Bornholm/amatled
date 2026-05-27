package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	amatled "github.com/bornholm/amatled"
	"github.com/bornholm/amatled/internal/app"
	"github.com/urfave/cli/v2"
)

// version est injecté à la compilation via -ldflags "-X main.version=vX.Y.Z".
var version = "dev"

func main() {
	logFile := setupLogger()
	if logFile != nil {
		defer logFile.Close()
	}

	application := &cli.App{
		Name:      "amatled",
		Usage:     "Éditeur Markdown desktop assisté par IA",
		ArgsUsage: "[répertoire]",
		Version:   version,
		Action: func(ctx *cli.Context) error {
			dir := ctx.Args().First()
			if dir == "" {
				dir = "."
			}
			abs, err := filepath.Abs(dir)
			if err != nil {
				return fmt.Errorf("chemin invalide : %w", err)
			}
			return app.New(version, abs).Run(amatled.WebFS)
		},
	}

	if err := application.Run(os.Args); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func setupLogger() *os.File {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil
	}
	logPath := filepath.Join(dir, "amatled", "amatled.log")
	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		return nil
	}
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil
	}
	w := io.MultiWriter(os.Stderr, f)
	slog.SetDefault(slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: slog.LevelInfo})))
	return f
}
