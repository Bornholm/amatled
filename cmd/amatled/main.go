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
	var logFile *os.File

	application := &cli.App{
		Name:      "amatled",
		Usage:     "Éditeur Markdown desktop assisté par IA",
		ArgsUsage: "[répertoire]",
		Version:   version,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "log-level",
				Usage: "Niveau de log (debug, info, warn, error)",
				Value: "info",
			},
		},
		Before: func(ctx *cli.Context) error {
			logFile = setupLogger(ctx.String("log-level"))
			return nil
		},
		After: func(ctx *cli.Context) error {
			if logFile != nil {
				return logFile.Close()
			}
			return nil
		},
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

// setupLogger configure le logger par défaut pour écrire à la fois sur stderr
// et dans un fichier de log utilisateur, au niveau donné (debug, info, warn, error).
// En cas de niveau invalide, retombe sur le niveau info.
func setupLogger(logLevel string) *os.File {
	level := slog.LevelInfo
	if logLevel != "" {
		if err := level.UnmarshalText([]byte(logLevel)); err != nil {
			slog.Warn("niveau de log invalide, utilisation du niveau info", "logLevel", logLevel, "err", err)
		}
	}

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
	slog.SetDefault(slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: level})))
	return f
}
