# AGENTS.md

This file provides guidance to LLM agents when working with code in this repository.

## Commands

### Build

```bash
# Build complet (frontend TS + binaire Go)
make build

# Frontend uniquement (TypeScript → web/dist/bundle.js)
npm run build

# Frontend en watch mode (développement)
npm run dev
```

### Run

```bash
# Build + lancer (sans argument = restaure le dernier workspace)
make run

# Mode développement (watch TS + go run en parallèle)
make dev

# Lancer sur un workspace spécifique
./bin/amatled /chemin/vers/workspace

# Lancer en ouvrant directement un fichier Markdown
./bin/amatled /chemin/vers/document.md
```

### Packaging

```bash
# Générer le paquet Pacman .pkg.tar.zst dans un conteneur Arch Linux
make package

# Installer le raccourci bureau localement (sans paquet)
make install-desktop
```

### Tests et lint

```bash
go test ./...                  # tous les tests Go
go test ./internal/history/... # un package spécifique
golangci-lint run ./...        # lint Go
```

## Architecture

L'application est un éditeur desktop basé sur **lorca** (fenêtre Chromium pilotée par Go). Le backend Go démarre un serveur HTTP local qui sert les assets web embarqués, puis ouvre la fenêtre avec l'URL correspondante.

### Communication Go ↔ JS

Le point d'entrée unique est `window.rpc(method, paramsJSON)` (lié dans `Bridge.Register`). Le Go répond avec `{ok: bool, value?, error?}`. Dans l'autre sens, Go pousse des événements via `window.__bus.emit(event, payload)` (`Bridge.Emit`).

Tous les handlers RPC sont enregistrés dans `internal/app/bridge.go` — c'est le fichier central pour ajouter ou modifier des fonctionnalités côté Go.

### Packages Go

| Package                      | Rôle                                                                                                                                  |
| ---------------------------- | ------------------------------------------------------------------------------------------------------------------------------------- |
| `internal/app`               | Lifecycle de l'app (`App`), serveur HTTP, ouverture lorca                                                                             |
| `internal/app` (`bridge.go`) | Tous les handlers RPC (workspace, editor, history, chat, settings, updater)                                                           |
| `internal/agent`             | Runner de l'agent IA : construit le prompt système, exécute la boucle `genai/agent/loop`, applique la normalisation Markdown post-run |
| `internal/editor`            | `Session` (contenu + historique d'un fichier ouvert) et `SessionManager`                                                              |
| `internal/history`           | Stack undo/redo avec opérations `ReplaceRange`, positions ligne/colonne, sources (Human vs Agent)                                     |
| `internal/document`          | Parser de sections Markdown (`ActiveSection`, `BuildWorkspaceIndex`)                                                                  |
| `internal/workspace`         | Lecture/écriture de fichiers, config workspace (`.amatled.yaml`)                                                                      |
| `internal/settings`          | Profils LLM multi-providers (config JSON) ; les secrets (API keys) sont dans le keyring système via `internal/keyring`                |
| `internal/render`            | Export PDF via le pipeline amatl (chromedp)                                                                                           |
| `internal/format`            | Normalisation Markdown (Goldmark)                                                                                                     |
| `internal/updater`           | Auto-update via `go-selfupdate` (GitHub releases)                                                                                     |

### Frontend TypeScript

Point d'entrée : `web/src/main.ts`. Bundlé par esbuild vers `web/dist/bundle.js` (embarqué dans le binaire Go via `//go:embed`).

- **CodeMirror 6** pour l'édition avec coloration syntaxique Markdown
- `web/src/bridge.ts` expose les fonctions `rpc()` et `bus` utilisées partout
- Le frontend est organisé en modules par feature dans `web/src/` (tree, tabs, editor, preview, chat, settings, toast…)

### Profils LLM

Les profils (`settings.Profile`) contiennent la config LLM (provider, baseURL, model, maxIterations, maxTokens) et un `SystemPrompt` optionnel. Les clés API sont **toujours** lues/écrites via `internal/keyring` (jamais stockées en clair dans le JSON). La résolution du profil actif est per-workspace (`.amatled.yaml`) avec fallback sur le global.

### Outils de l'agent

Définis dans `internal/agent/tools/` et enregistrés dans `Runner.buildTools()`. Ils opèrent sur un `SessionContext` (session courante + workspace) injecté dans le contexte Go.
