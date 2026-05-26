package updater

import (
	"context"
	"fmt"
	"log/slog"

	selfupdate "github.com/creativeprojects/go-selfupdate"
)

const repoSlug = "bornholm/amatled"

// Release enveloppe la release détectée (opaque pour les appelants).
type Release = selfupdate.Release

// Check interroge GitHub pour trouver une version plus récente que currentVersion.
// Retourne nil si déjà à jour ou si aucune release n'est trouvée.
func Check(ctx context.Context, currentVersion string) (*Release, error) {
	u, err := selfupdate.NewUpdater(selfupdate.Config{
		Validator: &selfupdate.ChecksumValidator{UniqueFilename: "checksums.txt"},
	})
	if err != nil {
		return nil, fmt.Errorf("create updater: %w", err)
	}
	latest, found, err := u.DetectLatest(ctx, selfupdate.ParseSlug(repoSlug))
	if err != nil {
		return nil, fmt.Errorf("detect latest: %w", err)
	}
	if !found {
		return nil, nil
	}
	if latest.LessOrEqual(currentVersion) {
		slog.Info("already up to date", "current", currentVersion, "latest", latest.Version())
		return nil, nil
	}
	slog.Info("update available", "current", currentVersion, "latest", latest.Version())
	return latest, nil
}

// Apply télécharge et remplace le binaire courant par la release donnée.
// L'utilisateur doit relancer l'application après l'appel.
func Apply(ctx context.Context, release *Release) error {
	u, err := selfupdate.NewUpdater(selfupdate.Config{
		Validator: &selfupdate.ChecksumValidator{UniqueFilename: "checksums.txt"},
	})
	if err != nil {
		return fmt.Errorf("create updater: %w", err)
	}
	exe, err := selfupdate.ExecutablePath()
	if err != nil {
		return fmt.Errorf("locate executable: %w", err)
	}
	if err := u.UpdateTo(ctx, release, exe); err != nil {
		return fmt.Errorf("apply update: %w", err)
	}
	slog.Info("update applied", "version", release.Version())
	return nil
}
