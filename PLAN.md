# Plan d'implémentation — AmatlEd

> Éditeur Markdown desktop avec assistant IA intégré, basé sur amatl et genai.

---

## 1. Vue d'ensemble

**AmatlEd** est une application desktop monobinaire écrite en Go, embarquant une UI HTML/JS via [lorca](https://github.com/zserge/lorca) (Chromium déjà installé sur le poste utilisateur). Elle propose :

- Un éditeur Markdown multi-onglets sur un workspace (dossier local)
- Deux vues commutables : **Source** (CodeMirror 6) et **Mise en forme** (rendu HTML amatl)
- Un assistant IA (genai) sous forme de chat latéral, capable de lire et modifier la **section active** du document (détection par position du curseur, portée au sens amatl)
- Un système d'historique unifié permettant de rollback toute modification (humaine ou IA)
- Une distribution binaire avec autoupdate

### Décisions structurantes

| # | Décision |
|---|---|
| 1 | Édition IA directe, rollback via historique |
| 2 | Section active = position curseur, portée amatl |
| 3 | CodeMirror 6 |
| 4 | Sauvegarde manuelle (Ctrl+S) |
| 5 | UI Paramètres + secrets via `go-keyring` |
| 6 | Un fil de chat global, persisté en SQLite (user data dir) |
| 7 | MCP en v2 (tools internes only en v1) |
| 8 | Re-render au switch de vue |
| 9 | Lorca (fork existant) + autoupdate via `creativeprojects/go-selfupdate` |
| 10 | IA écrit uniquement dans le fichier actif |
| 11 | Dialogs natifs Go (`sqweek/dialog`) |
| 12 | Nom **AmatlEd**, thème dark VS Code-like |

---

## 2. Stack technique

### Backend (Go 1.25)

| Dépendance | Usage |
|---|---|
| Fork de `github.com/zserge/lorca` dans `../lorca` | Shell Chromium |
| `github.com/bornholm/amatl/...` | Parsing/rendu Markdown, résolveurs, pipeline |
| `github.com/bornholm/genai/...` | Client LLM, agent loop, tool calling, streaming |
| `github.com/yuin/goldmark` | AST Markdown (déjà tiré par amatl) |
| `modernc.org/sqlite` | Persistance chat & métadonnées |
| `github.com/zalando/go-keyring` | Secrets (clés API) |
| `github.com/sqweek/dialog` | File pickers natifs |
| `github.com/creativeprojects/go-selfupdate` | Autoupdate |
| `github.com/urfave/cli/v2` | CLI |
| `log/slog` | Logs structurés |

### Frontend

- HTML5 + CSS3 (custom, palette VS Code Dark+)
- TypeScript compilé en bundle unique (esbuild)
- CodeMirror 6 (`@codemirror/lang-markdown`, `@codemirror/view`, `@codemirror/state`)
- Pas de framework lourd (Vanilla TS + petit event bus typé)

### Build & distribution

- `make build` → binaire unique (front bundlé embarqué via `embed.FS`)
- `goreleaser` → release multi-OS (Linux d'abord)
- `go-selfupdate` → check au démarrage, prompt utilisateur si nouvelle version

---

## 3. Architecture

### Arborescence du projet

```
amatled/
├── cmd/amatled/
│   └── main.go                  # entrypoint, CLI, lorca bootstrap
├── internal/
│   ├── app/                     # orchestration globale
│   │   ├── app.go               # struct App, lifecycle
│   │   └── bridge.go            # RPC Go ↔ JS
│   ├── workspace/               # ouverture dossier, arborescence, watcher
│   ├── document/                # modèle de document, parsing amatl
│   │   ├── document.go
│   │   └── section.go           # détection section active
│   ├── history/                 # historique unifié + undo/redo
│   │   ├── stack.go
│   │   └── ops.go
│   ├── editor/                  # session d'édition par fichier
│   ├── chat/                    # session de chat, persistance SQLite
│   │   ├── store.go
│   │   └── session.go
│   ├── agent/                   # wrapper genai + tools internes
│   │   ├── runner.go
│   │   └── tools/
│   │       ├── read_section.go
│   │       ├── replace_section.go
│   │       ├── insert_after.go
│   │       └── ...
│   ├── render/                  # rendu HTML via amatl
│   ├── settings/                # config + keyring
│   ├── updater/                 # autoupdate
│   └── dialog/                  # wrappers dialog natif
├── web/
│   ├── src/
│   │   ├── main.ts
│   │   ├── bridge.ts            # client RPC
│   │   ├── editor/              # intégration CodeMirror
│   │   ├── chat/
│   │   ├── tabs/
│   │   ├── tree/
│   │   ├── settings/
│   │   └── styles/
│   ├── index.html
│   └── tsconfig.json
├── misc/
│   ├── make/
│   └── build/
├── Makefile
├── go.mod
└── README.md
```

### Diagramme de flux principal

```
┌─────────────────────────────────────────────────────────────┐
│                     UI (Chromium / lorca)                    │
│  ┌──────────┐  ┌─────────────────┐  ┌─────────────────────┐ │
│  │   Tree   │  │  Editor (CM6)    │  │   Chat panel        │ │
│  │          │  │  Source / Preview│  │  + Section lock 🔒  │ │
│  └─────┬────┘  └────────┬─────────┘  └──────────┬──────────┘ │
└────────┼────────────────┼───────────────────────┼─────────────┘
         │ rpc("openFile")│ rpc("applyEdit")      │ rpc("sendMessage")
         │ evt(...)       │ evt("docChanged")     │ evt("agentDelta")
┌────────▼────────────────▼───────────────────────▼─────────────┐
│                         Bridge (Go)                           │
└────────┬────────────────┬───────────────────────┬─────────────┘
         │                │                       │
    ┌────▼────┐    ┌──────▼──────┐         ┌──────▼──────┐
    │workspace│    │  editor     │◄────────┤   agent     │
    └─────────┘    │  + document │ tools   │  (genai)    │
                   │  + history  │         └──────┬──────┘
                   └──────┬──────┘                │
                          │                  ┌────▼────┐
                          │                  │  chat   │
                          │                  │  store  │
                          ▼                  └─────────┘
                   ┌─────────────┐
                   │   render    │ (amatl pipeline)
                   └─────────────┘
```

### Bridge Go ↔ JS

Un unique point d'entrée bindé `rpc(method, paramsJSON)` qui dispatche vers des handlers Go. Pour les événements Go → JS (streaming, notifications), un event bus :

```go
// internal/app/bridge.go
type Bridge struct {
    ui      lorca.UI
    handlers map[string]Handler
}

func (b *Bridge) Emit(event string, payload any) {
    data, _ := json.Marshal(payload)
    b.ui.Eval(fmt.Sprintf("window.__bus.emit(%q, %s)", event, data))
}
```

Côté JS :

```ts
// web/src/bridge.ts
export async function rpc<T>(method: string, params: unknown): Promise<T> {
  const res = await (window as any).rpc(method, JSON.stringify(params));
  const { ok, value, error } = JSON.parse(res);
  if (!ok) throw new Error(error);
  return value as T;
}

export const bus = new EventBus(); // window.__bus
```

---

## 4. Roadmap par phases

### Phase 0 — Bootstrap (1-2 j)

- Skeleton projet, Makefile, lint
- Bootstrap lorca + page blanche
- Bridge RPC bidirectionnel minimal + event bus
- Pipeline TS (esbuild) + embed
- CI GitHub Actions (build Linux/macOS/Windows)

**Critère sortie** : `make run` ouvre une fenêtre, un `ping` JS → Go → JS fonctionne.

---

### Phase 1 — Workspace & UI shell (2-3 j)

- Sélection dossier via `sqweek/dialog`
- Lecture arborescence (filtrée : `.md` only, ignore `node_modules`, `.git`)
- Vue 3 colonnes (tree | editor | chat) en CSS Grid
- Thème dark VS Code-like (variables CSS palette `--bg`, `--fg`, `--accent`, etc.)
- Onglets multi-fichiers (open, close, dirty indicator, switch)
- Persistance du dernier workspace ouvert dans la config

**Critère sortie** : on ouvre un dossier, on voit l'arbre, on peut ouvrir plusieurs fichiers en onglets (contenu brut affiché en `<pre>`).

---

### Phase 2 — Historique unifié + modèle document (3-4 j)

**Phase critique** détaillée en section 5.

- Modèle `Document` (contenu + métadonnées)
- Stack d'historique par fichier, opérations atomiques
- Détection section active via parser goldmark
- Undo/redo global (Ctrl+Z, Ctrl+Shift+Z)
- Désactivation de l'historique natif CM6, routage via Go

**Critère sortie** : taper du texte, Ctrl+Z annule par groupes cohérents ; appel programmatique `ApplyOp` depuis le test fonctionne.

---

### Phase 3 — Éditeur CodeMirror 6 (2-3 j)

- Intégration CM6 avec extensions Markdown
- Coloration syntaxique adaptée au thème dark
- Binding bidirectionnel : modifs CM6 → Go (debounced 200ms) → history.Push
- Replays inverses : Go → CM6 (sur undo/redo ou édition IA) sans boucle infinie
- Indicateur visuel de la section active (gutter ou highlight bord gauche)
- Sauvegarde Ctrl+S avec confirmation visuelle
- Prompt fermeture si dirty

**Critère sortie** : édition fluide, Ctrl+S persiste, indicateur de section active suit le curseur.

---

### Phase 4 — Rendu Mise en forme (2 j)

- Toggle Source/Preview au niveau onglet
- Au switch vers Preview : appel pipeline amatl (programmatique, pas CLI)
- Affichage dans une `<iframe sandbox>` ou un conteneur isolé
- Thème dark du rendu (CSS injecté)
- Spinner durant le rendu

**Critère sortie** : on bascule vue, le rendu apparaît, les liens internes fonctionnent, basique mais propre.

---

### Phase 5 — Chat & agent IA (4-6 j) — cœur du produit

#### 5.1 UI chat
- Liste de messages (user/assistant), markdown rendu
- Input multiligne, envoi Ctrl+Enter
- Indicateur "section verrouillée" 🔒 : nom de la section incluse dans le contexte
- Streaming : tokens apparaissent au fil de l'eau
- Visualisation des appels d'outils (collapsible : "🔧 replace_section → ✓")
- Bouton "Annuler" durant un run

#### 5.2 Settings
- Écran modal : provider, baseURL, model, maxIterations, maxTokens
- Clé API stockée via `go-keyring` (service: `amatled`, account: `<provider>`)
- Test de connexion bouton

#### 5.3 Agent runner

Wrapper autour de `genai/agent/loop` :

```go
type Runner struct {
    client    llm.ChatCompletionStreamingClient
    editor    *editor.Session  // session active uniquement
    history   *history.Stack
    tools     []tool.Tool
    mu        sync.Mutex
}

func (r *Runner) Run(ctx context.Context, userMsg string, section *document.SectionRef) (<-chan Event, error)
```

Lock mutex durant un run → un seul agent run actif à la fois. Éditeur passe en read-only.

#### 5.4 Tools internes v1

| Tool | Description | Portée |
|---|---|---|
| `read_section` | Lit le contenu Markdown de la section verrouillée | RO |
| `read_document` | Lit le document entier (cap tokens) | RO |
| `replace_section` | Remplace le contenu de la section verrouillée | RW |
| `insert_before_section` | Insère un bloc avant la section | RW |
| `insert_after_section` | Insère un bloc après la section | RW |
| `list_sections` | Liste headings du document | RO |
| `get_section_by_title` | Récupère une autre section (RO) | RO |

Chaque tool RW :
1. Récupère la session active
2. Construit un `history.Op` typé `Source=Agent` avec `aiMessageId`
3. Appelle `history.Push(op)` qui applique et notifie l'UI
4. Retourne un diff résumé à l'IA

#### 5.5 Persistance chat (SQLite)

Localisation : `os.UserConfigDir()/AmatlEd/data.db`

Schéma :

```sql
CREATE TABLE messages (
  id TEXT PRIMARY KEY,
  workspace TEXT NOT NULL,
  role TEXT NOT NULL,            -- user | assistant | tool
  content TEXT NOT NULL,
  timestamp DATETIME NOT NULL,
  linked_history_id TEXT,
  metadata JSON
);
CREATE INDEX idx_msg_workspace_ts ON messages(workspace, timestamp);

CREATE TABLE history_entries (
  id TEXT PRIMARY KEY,
  workspace TEXT NOT NULL,
  file_path TEXT NOT NULL,
  timestamp DATETIME NOT NULL,
  source TEXT NOT NULL,           -- human | agent
  ai_message_id TEXT,
  op_type TEXT NOT NULL,
  op_data JSON NOT NULL,          -- payload sérialisé
  inverse_data JSON NOT NULL
);
CREATE INDEX idx_hist_workspace_file ON history_entries(workspace, file_path);
```

**Critère sortie** : conversation streamée fonctionnelle, l'IA peut modifier la section active, modifs annulables via Ctrl+Z (depuis l'éditeur) ou bouton "rollback" sur le message dans le chat.

---

### Phase 6 — Polish & robustesse (2-3 j)

- Gestion erreurs : toast système, log file
- Indicateurs réseau (provider down, rate limit)
- Raccourcis clavier complets (cf SPEC)
- Accessibilité : contraste, focus trap modal, navigation clavier chat
- File watcher pour rafraîchir l'arbre si modifs externes
- Tests E2E principaux scénarios (chromedp piloté sur le binaire ?)

---

### Phase 7 — Distribution & autoupdate (1-2 j)

- `goreleaser` config (Linux x86_64 prioritaire)
- Intégration `creativeprojects/go-selfupdate` :
  - Source : releases GitHub
  - Check au démarrage + bouton manuel dans Paramètres
  - Vérif signature GPG des binaires
- README + screenshots
- Icône application

---

## 5. Détail Phase 2 — Historique unifié

Cette phase est **structurante** : un mauvais design ici se paye sur toutes les phases suivantes.

### 5.1 Principes

1. **Toute mutation passe par le stack**, sans exception (frappe user, IA, paste, find/replace).
2. **CM6 history désactivé**, on intercepte ses transactions et on les route vers Go.
3. **Coalescing** : les frappes utilisateur consécutives sont agrégées par fenêtre temporelle (500 ms) ou rupture sémantique (fin de mot, retour ligne).
4. **Actions IA jamais coalescées** : un tool call RW = exactement une entrée.
5. **Persistance** : entrées sérialisées en SQLite pour rollback même après redémarrage (optionnel v1, design ready).

### 5.2 Types Go

```go
// internal/history/ops.go
package history

type Source string

const (
    SourceHuman Source = "human"
    SourceAgent Source = "agent"
)

type Position struct {
    Line   int // 0-indexed
    Column int // 0-indexed, en runes
}

type Op interface {
    Type() string
    Apply(content string) (string, error)
    Inverse(content string) (Op, error)
    Describe() string
}

// Op atomique : remplace une plage par une nouvelle chaîne
type ReplaceRange struct {
    Start, End Position
    NewText    string
}

func (o *ReplaceRange) Type() string { return "replace_range" }

func (o *ReplaceRange) Apply(content string) (string, error) {
    startOff, err := offsetOf(content, o.Start)
    if err != nil {
        return "", err
    }
    endOff, err := offsetOf(content, o.End)
    if err != nil {
        return "", err
    }
    if startOff > endOff || endOff > len(content) {
        return "", ErrInvalidRange
    }
    return content[:startOff] + o.NewText + content[endOff:], nil
}

func (o *ReplaceRange) Inverse(content string) (Op, error) {
    startOff, _ := offsetOf(content, o.Start)
    endOff, _ := offsetOf(content, o.End)
    oldText := content[startOff:endOff]
    // Calculer la position de fin après application
    endAfter := positionAfter(content[:startOff] + o.NewText)
    return &ReplaceRange{
        Start:   o.Start,
        End:     endAfter,
        NewText: oldText,
    }, nil
}

func (o *ReplaceRange) Describe() string {
    return fmt.Sprintf("replace %v-%v with %d chars", o.Start, o.End, len(o.NewText))
}
```

### 5.3 Stack

```go
// internal/history/stack.go
package history

type Entry struct {
    ID            string
    Timestamp     time.Time
    Source        Source
    AIMessageID   string // si Source=Agent
    Op            Op
    InverseOp     Op
    Coalescable   bool
}

type Stack struct {
    mu       sync.Mutex
    entries  []Entry
    cursor   int            // index de la prochaine entrée à pousser
    content  string         // snapshot courant (source of truth)
    capacity int            // ex 500
    lastPush time.Time
    onChange func(content string, entry Entry, direction Direction)
}

type Direction int

const (
    DirForward Direction = iota
    DirUndo
    DirRedo
)

func New(initial string, onChange func(string, Entry, Direction)) *Stack {
    return &Stack{
        content:  initial,
        capacity: 500,
        onChange: onChange,
    }
}

func (s *Stack) Content() string {
    s.mu.Lock()
    defer s.mu.Unlock()
    return s.content
}

func (s *Stack) Push(op Op, src Source, aiMsgID string) error {
    s.mu.Lock()
    defer s.mu.Unlock()

    inv, err := op.Inverse(s.content)
    if err != nil {
        return err
    }
    newContent, err := op.Apply(s.content)
    if err != nil {
        return err
    }

    now := time.Now()
    entry := Entry{
        ID:          newID(),
        Timestamp:   now,
        Source:      src,
        AIMessageID: aiMsgID,
        Op:          op,
        InverseOp:   inv,
        Coalescable: src == SourceHuman,
    }

    // Tronque la branche redo
    s.entries = s.entries[:s.cursor]

    // Coalescing : si dernière entrée humaine et < 500ms et même type
    if s.cursor > 0 && entry.Coalescable {
        last := &s.entries[s.cursor-1]
        if last.Coalescable && now.Sub(last.Timestamp) < 500*time.Millisecond {
            if merged, ok := tryMerge(last.Op, op); ok {
                last.Op = merged
                last.Timestamp = now
                // Recalculer inverse à partir du content pré-merge nécessite snapshot ;
                // alternative simple : ne pas merger les inverses, garder le 1er.
                s.content = newContent
                s.onChange(s.content, *last, DirForward)
                return nil
            }
        }
    }

    s.entries = append(s.entries, entry)
    s.cursor++
    s.content = newContent

    // Trim capacity
    if len(s.entries) > s.capacity {
        drop := len(s.entries) - s.capacity
        s.entries = s.entries[drop:]
        s.cursor -= drop
    }

    s.onChange(s.content, entry, DirForward)
    return nil
}

func (s *Stack) Undo() (*Entry, error) {
    s.mu.Lock()
    defer s.mu.Unlock()

    if s.cursor == 0 {
        return nil, ErrNothingToUndo
    }
    entry := s.entries[s.cursor-1]
    newContent, err := entry.InverseOp.Apply(s.content)
    if err != nil {
        return nil, err
    }
    s.content = newContent
    s.cursor--
    s.onChange(s.content, entry, DirUndo)
    return &entry, nil
}

func (s *Stack) Redo() (*Entry, error) {
    s.mu.Lock()
    defer s.mu.Unlock()

    if s.cursor >= len(s.entries) {
        return nil, ErrNothingToRedo
    }
    entry := s.entries[s.cursor]
    newContent, err := entry.Op.Apply(s.content)
    if err != nil {
        return nil, err
    }
    s.content = newContent
    s.cursor++
    s.onChange(s.content, entry, DirRedo)
    return &entry, nil
}

// RollbackTo annule toutes les entrées jusqu'à `entryID` inclus (utilisé
// par le bouton "rollback" sur un message IA dans le chat).
func (s *Stack) RollbackTo(entryID string) error {
    s.mu.Lock()
    defer s.mu.Unlock()

    target := -1
    for i := s.cursor - 1; i >= 0; i-- {
        if s.entries[i].ID == entryID {
            target = i
            break
        }
    }
    if target < 0 {
        return ErrEntryNotFound
    }

    for s.cursor > target {
        entry := s.entries[s.cursor-1]
        newContent, err := entry.InverseOp.Apply(s.content)
        if err != nil {
            return err
        }
        s.content = newContent
        s.cursor--
        s.onChange(s.content, entry, DirUndo)
    }
    return nil
}
```

### 5.4 Modèle Document & détection section

```go
// internal/document/section.go
package document

type SectionRef struct {
    HeadingLevel int
    HeadingTitle string
    StartLine    int      // ligne du heading
    EndLine      int      // ligne avant le prochain heading <= level (ou EOF)
    RawContent   string   // contenu Markdown brut, heading inclus
}

// ActiveSection retourne la section contenant la ligne donnée,
// au sens amatl : section = heading + tout jusqu'au prochain heading
// de niveau <= au sien.
func ActiveSection(content string, cursorLine int) (*SectionRef, error) {
    md := goldmark.New(goldmark.WithExtensions(/* extensions amatl */))
    reader := text.NewReader([]byte(content))
    doc := md.Parser().Parse(reader)

    var headings []*ast.Heading
    ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
        if !entering {
            return ast.WalkContinue, nil
        }
        if h, ok := n.(*ast.Heading); ok {
            headings = append(headings, h)
        }
        return ast.WalkContinue, nil
    })

    // Trouver le heading dont la ligne <= cursorLine et qui "domine" cursorLine
    // (i.e. pas de heading de niveau <= entre ce heading et cursorLine)
    // ...

    return ref, nil
}
```

### 5.5 Synchronisation CodeMirror ↔ Go

**Côté CM6** : on désactive `history()` du package `@codemirror/commands`. On installe un `EditorView.updateListener` qui capture les `Transaction` produites par l'utilisateur (filtrées sur `userEvent`).

```ts
// web/src/editor/sync.ts
const updateListener = EditorView.updateListener.of((update) => {
  if (!update.docChanged) return;
  if (update.transactions.some(t => t.annotation(remoteAnnotation))) {
    return; // édition venant de Go, ne pas re-router
  }
  const changes: Change[] = [];
  update.changes.iterChanges((fromA, toA, fromB, toB, inserted) => {
    changes.push({
      from: posToLineCol(update.startState.doc, fromA),
      to:   posToLineCol(update.startState.doc, toA),
      insert: inserted.toString(),
    });
  });
  rpc("editor.applyLocalChanges", { fileId, changes });
});
```

Quand Go pousse un changement (undo/redo/IA), il envoie un événement `editor.remoteChange` que le front applique avec `remoteAnnotation` pour éviter la boucle :

```ts
view.dispatch({
  changes: [...],
  annotations: remoteAnnotation.of(true),
});
```

**Ctrl+Z** est intercepté par un keymap CM6 personnalisé qui appelle `rpc("history.undo")`.

---

## 6. Modèle de données (récapitulatif)

### Côté mémoire

```
App
├── Workspace
│   ├── path
│   └── files: []FileTab
└── ChatSession
    └── messages: []ChatMessage

FileTab
├── path
├── isDirty
├── activeSection: SectionRef
├── sectionLocked: bool
└── history: *history.Stack
```

### Côté SQLite (`<user_config_dir>/amatled/data.db`)

- `messages` : historique chat global
- `history_entries` : trace des ops (snapshot léger, support rollback cross-restart)
- `workspaces` : derniers workspaces ouverts, dernier fichier actif par workspace
- `settings` : kv des préférences non secrètes (clé API → keyring)

---

## 7. Sécurité

- **Filesystem** : tous les paths résolus relativement au workspace, vérif `filepath.Rel` pour éviter le traversal. L'agent ne peut écrire que dans le fichier actif (vérif au niveau du tool).
- **Secrets** : `go-keyring` service `amatled`, account `llm.<provider>`. Fallback explicite si keyring indisponible : prompt utilisateur, refus de stocker en clair.
- **CSP** : `<iframe sandbox="allow-same-origin">` pour le rendu preview, pas de `allow-scripts` (sauf si Mermaid l'exige — à vérifier).
- **Updater** : vérif signature GPG (clé publique embarquée dans le binaire).

---

## 8. Performance — cibles SPEC

| Cible | Stratégie |
|---|---|
| Ouverture < 500ms (500 Ko) | Lecture sync, parsing goldmark lazy |
| Détection section < 50ms | Cache de l'AST, invalidation sur edit, recalcul async débouncé |
| Rendu < 5s | Pipeline amatl programmatique (pas de fork CLI) |
| 1er token chat < 1s | Streaming SSE direct, pas de buffering |
| 50 000 mots fluides | Coalescing historique, debounce 200ms sur sync CM6 → Go |
| 200 fichiers workspace | Tree virtualisé si > 100 entrées |

---

## 9. Tests

- **Unit** : `history` (push/undo/redo/coalesce/rollback), `document.ActiveSection` (table-driven sur fixtures Markdown)
- **Intégration** : agent runner avec mock LLM, vérif tools modifient bien le doc
- **E2E** : pilotage du binaire via chromedp sur scénarios clés (ouvrir → éditer → IA modifie → undo)
- **Fixtures** : `testdata/` avec fichiers Markdown variés (sections imbriquées, frontmatter, code blocks)

---

## 10. Risques & mitigations

| Risque | Probabilité | Impact | Mitigation |
|---|---|---|---|
| Double-historique CM6/Go incohérent | Moyenne | Élevé | CM6 history désactivé dès Phase 2 |
| CGO sqlite casse cross-compile | Élevée | Moyen | Fallback `modernc.org/sqlite` |
| Fork lorca non maintenu | Faible | Élevé | Abstraction `Bridge`, plan B Wails documenté |
| Sélecteur section ambigu (mêmes titres) | Moyenne | Faible | Index ordinal en complément du titre |
| Conflit édition pendant tool call | Élevée | Moyen | Éditeur read-only durant agent run |
| LLM hallucine sélecteur | Élevée | Faible | Tools renvoient erreur explicite, retry |
| Keyring indisponible (Linux headless) | Faible | Faible | Prompt + refus stockage, mode "session" |

---

## 11. Estimation

| Phase | Durée |
|---|---|
| 0. Bootstrap | 1-2 j |
| 1. Workspace & shell | 2-3 j |
| 2. Historique & document | 3-4 j |
| 3. Éditeur CM6 | 2-3 j |
| 4. Rendu Mise en forme | 2 j |
| 5. Chat & agent | 4-6 j |
| 6. Polish | 2-3 j |
| 7. Distribution & autoupdate | 1-2 j |
| **Total MVP** | **17-25 j** |

---

## 12. Out of scope v1

- MCP externes
- Multi-fil de chat
- Édition cross-fichier par l'IA
- Sauvegarde automatique
- Plugin system
- Mode collaboratif
- Recherche full-text dans le workspace
- Git integration (status, diff)
- Export PDF direct depuis l'UI (déjà faisable via CLI amatl)
