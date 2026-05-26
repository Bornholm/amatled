# Spécification Fonctionnelle — Éditeur Markdown Desktop Assisté par IA

---

## Feature Overview

- **Feature Name** : Amatl Editor — Éditeur Markdown Desktop Assisté par IA
- **Business Value** : Permettre à un auteur de rédiger des documents longs en Markdown avec l'assistance d'un agent IA capable de comprendre la structure du document et d'intervenir chirurgicalement sur des sections précises, dans un flux de travail collaboratif fluide et non intrusif.
- **Priority** : High
- **Estimated Complexity** : High — Architecture multi-couches (Go backend, HTML5 frontend, intégration amatl + genai + lorca), gestion d'état complexe (sélection active, historique hybride humain/IA, multi-documents).

---

## User Stories

1. **As a** writer, **I want to** open and edit Markdown files with syntax highlighting **so that** I can work on long-form content comfortably.
2. **As a** writer, **I want to** see my document rendered with the full amatl pipeline **so that** I can preview the final result including diagrams, directives, and table of contents.
3. **As a** writer, **I want to** select a section of my document and ask the AI agent to improve it **so that** I can get assistance without leaving my editor.
4. **As a** writer, **I want to** the AI's changes to be applied immediately but identifiable in the history **so that** I can roll back or amend them if needed.
5. **As a** writer, **I want to** configure my LLM provider from within the app **so that** I don't need to manually edit config files.
6. **As a** writer, **I want to** work on multi-file documents with amatl inclusions **so that** large documents with cross-references are fully supported.

---

## Functional Requirements

### Éditeur

1. L'application doit permettre d'ouvrir plusieurs fichiers Markdown simultanément, présentés sous forme d'onglets dans la colonne d'édition.
2. Chaque fichier ouvert dispose de deux sous-onglets : **Source** (édition brute) et **Mise en forme** (rendu amatl).
3. L'éditeur Source doit proposer de la coloration syntaxique Markdown (GFM).
4. Le rendu "Mise en forme" est déclenché à la demande (bouton ou raccourci clavier) et utilise le pipeline amatl complet (MermaidJS, directives, tables des matières, Go templating, etc.).
5. L'éditeur doit supporter l'ouverture d'un **workspace** (dossier racine) afin de résoudre correctement les inclusions amatl inter-fichiers.
6. Les fichiers peuvent être ouverts, créés, sauvegardés et sauvegardés-sous depuis le système de fichiers local via des boîtes de dialogue natives.
7. L'éditeur doit maintenir un **historique d'annulation/rétablissement unifié** (Ctrl+Z / Ctrl+Y) qui couvre à la fois les modifications humaines et les modifications IA.
8. Chaque entrée d'historique produite par l'IA doit être **annotée** comme telle (auteur = "IA", timestamp, résumé de la demande ayant déclenché la modification).

### Sélection Active

9. L'éditeur doit exposer en permanence la **section active**, définie comme le bloc structurel (paragraphe, section de titre, liste, bloc de code, directive, etc.) dans lequel se trouve le curseur.
10. La section active doit être visuellement mise en évidence dans l'éditeur Source (surlignage de gouttière ou bordure latérale).
11. Le nom de la section active (ex : titre de la section parente) doit être affiché dans la colonne Chat comme contexte courant.
12. L'utilisateur doit pouvoir **verrouiller** la sélection active sur une section précise (pour éviter qu'elle change lorsqu'il déplace le curseur pendant une conversation avec l'IA).

### Interface Chat

13. La colonne Chat doit permettre d'envoyer des messages texte à l'agent IA.
14. Les réponses de l'agent doivent être affichées avec leur contenu textuel et, si l'agent a effectué une modification, un résumé de l'action réalisée.
15. Le Chat doit afficher clairement la **section active courante** sur laquelle l'agent travaille.
16. L'utilisateur doit pouvoir depuis le Chat :
    - Demander à l'IA d'**annuler** sa dernière modification (rollback).
    - Demander à l'IA d'**amender** sa modification précédente.
    - Consulter un **résumé de l'historique** des actions IA sur la session.
17. Le Chat doit supporter le **streaming** des réponses de l'agent (affichage token par token).
18. Une nouvelle **conversation** peut être initiée à tout moment, effaçant le contexte conversationnel sans affecter le document.

### Agent IA

19. L'agent reçoit en contexte initial :
    - La **table des matières** du document actif (générée par amatl).
    - Le **contenu de la section active** (texte Markdown brut, incluant les directives amatl).
    - Le **nom et chemin du fichier** actif.
20. L'agent doit disposer des **outils suivants** :
    - `get_section(selector)` — Récupère le contenu d'une section via un sélecteur CSS amatl (ex: `h2:contains("Introduction")`).
    - `replace_section(selector, new_content)` — Remplace le contenu d'une section identifiée par son sélecteur.
    - `insert_after(selector, content)` — Insère du contenu après une section identifiée.
    - `insert_before(selector, content)` — Insère du contenu avant une section identifiée.
    - `get_toc()` — Retourne la table des matières complète du document.
    - `search_document(query)` — Recherche textuelle dans l'ensemble des fichiers du workspace.
    - `get_file(path)` — Lit le contenu d'un fichier du workspace.
21. Toute modification du document par l'agent doit transiter par le système d'historique unifié (exigence #8).
22. L'agent doit être capable de manipuler les directives amatl (`:include`, `:toc`, `:attrs`, etc.) dans le contenu qu'il produit.

### Configuration LLM

23. Une interface de paramètres accessible depuis le menu de l'application doit permettre de configurer :
    - Le **provider** LLM (liste des providers supportés par genai).
    - L'**URL de base** de l'API.
    - La **clé API**.
    - Le **modèle**.
    - Les paramètres avancés : `maxIterations`, `maxTokens`, `reasoningEffort`.
24. La configuration doit être persistée localement (fichier de configuration utilisateur).
25. L'application doit signaler clairement si aucune configuration LLM valide n'est détectée au démarrage.

---

## User Flow

### Flux principal — Édition assistée

```
1. L'utilisateur lance l'application
   → Si pas de config LLM : bannière d'avertissement + lien vers les paramètres
   
2. L'utilisateur ouvre un workspace (dossier) ou un fichier individuel
   → Les fichiers s'ouvrent dans des onglets
   → L'arborescence du workspace est affichée (optionnel v1 : panneau latéral)

3. L'utilisateur édite dans l'onglet Source
   → La section active est détectée en temps réel via la position du curseur
   → La section active est mise en évidence + affichée dans la colonne Chat

4. L'utilisateur décide de demander l'aide de l'IA
   → Il peut verrouiller la sélection active si besoin
   → Il saisit sa demande dans le Chat et envoie

5. L'agent IA reçoit : ToC + section active + message utilisateur
   → Il raisonne, appelle des tools si nécessaire (get_section, search_document...)
   → Il applique les modifications via replace_section / insert_after / etc.
   → Les modifications apparaissent en temps réel dans l'éditeur Source
   → La réponse de l'agent s'affiche en streaming dans le Chat

6. L'utilisateur évalue le résultat
   → Option A : Satisfait → continue à éditer
   → Option B : Demande un amendement → nouveau message dans le Chat
   → Option C : Rollback → "annuler la dernière modification IA" (Ctrl+Z ou bouton Chat)

7. L'utilisateur passe en onglet Mise en forme pour prévisualiser
   → Rendu amatl complet déclenché à la demande
```

### Flux alternatif — Rollback IA

```
1. L'utilisateur envoie "annule ta dernière modification" dans le Chat
   OU utilise Ctrl+Z qui atterrit sur une entrée annotée [IA]
   
2. Le document revient à l'état précédant la modification IA
3. Le Chat affiche : "Modification annulée. Que souhaitez-vous faire ?"
4. L'utilisateur peut soit reformuler sa demande, soit continuer à éditer
```

---

## Acceptance Criteria

**Éditeur de base**

- **Given** un fichier Markdown sur le système de fichiers, **When** l'utilisateur l'ouvre, **Then** son contenu apparaît dans l'onglet Source avec coloration syntaxique en moins de 500ms.
- **Given** un document ouvert, **When** l'utilisateur clique sur l'onglet Mise en forme, **Then** le rendu amatl complet est affiché (MermaidJS, directives résolues, ToC générée).
- **Given** un document avec une directive `:include`, **When** le rendu est déclenché, **Then** le fichier inclus est résolu relativement au workspace racine.

**Sélection active**

- **Given** l'éditeur Source ouvert, **When** le curseur est positionné dans un paragraphe sous un titre H2, **Then** la section active affichée dans le Chat correspond à ce titre H2.
- **Given** une sélection verrouillée, **When** l'utilisateur déplace le curseur, **Then** la section active dans le Chat ne change pas.

**Agent IA**

- **Given** une section active sélectionnée et un message utilisateur envoyé, **When** l'agent répond, **Then** le contenu de la section dans l'éditeur est modifié dans les 30 secondes (hors latence réseau LLM).
- **Given** l'agent a modifié le document, **When** l'utilisateur effectue Ctrl+Z, **Then** le document revient à l'état précédant la modification, et l'entrée d'historique annulée est annotée [IA].
- **Given** l'agent utilise l'outil `get_section`, **When** le sélecteur CSS ne correspond à aucune section, **Then** l'outil retourne une erreur descriptive que l'agent peut interpréter.

**Configuration**

- **Given** aucune clé API configurée, **When** l'utilisateur tente d'envoyer un message dans le Chat, **Then** un message d'erreur explicite est affiché avec un lien vers les paramètres.
- **Given** une configuration LLM valide, **When** l'utilisateur la modifie dans les paramètres, **Then** la nouvelle configuration est prise en compte sans redémarrage de l'application.

---

## Edge Cases & Error Handling

| Scénario | Comportement attendu |
|---|---|
| L'agent tente de modifier une section qui a été éditée manuellement entre-temps | Avertissement : "La section a été modifiée depuis le début de la requête. Appliquer quand même ?" |
| Le fichier est modifié sur le disque par un éditeur externe | Bannière de rechargement : "Le fichier a changé sur le disque. Recharger ?" |
| L'appel LLM échoue (timeout, quota, erreur réseau) | Message d'erreur dans le Chat avec possibilité de réessayer ; le document n'est pas modifié |
| L'agent produit du Markdown syntaxiquement invalide | Affichage dans l'éditeur tel quel, avertissement visuel ; l'utilisateur peut rollback |
| Le workspace contient des fichiers non-Markdown | Ils sont ignorés par l'agent mais visibles dans l'arborescence |
| La section active est un bloc de code ou une directive complexe | L'agent en est informé explicitement dans son contexte |
| Document vide | La ToC est vide, la section active est le document entier |
| Sélecteur CSS de l'agent ne matche rien | L'outil retourne `{"error": "No element matching selector '<selector>'"}` |
| Fichier non sauvegardé lors de la fermeture | Dialogue de confirmation : "Sauvegarder avant de fermer ?" |
| Plusieurs modifications IA en file d'attente (requêtes rapides successives) | Les requêtes sont sérialisées ; une seule modification IA active à la fois |

---

## Non-Functional Requirements

### Performance
- Ouverture d'un fichier jusqu'à 500 Ko : < 500ms
- Détection de la section active à chaque mouvement de curseur : < 50ms
- Déclenchement du rendu amatl : feedback visuel (spinner) immédiat, rendu complet < 5s pour des documents standards
- Le streaming des réponses IA doit apparaître dans le Chat en < 1s après le premier token reçu

### Sécurité
- L'accès aux fichiers via les outils de l'agent est strictement confiné au workspace ouvert (même mécanique que `amatl mcp serve --workspace`)
- Les clés API sont stockées dans le keychain système (via une librairie Go appropriée) et non en clair dans un fichier de config
- Aucune donnée du document n'est transmise à des services tiers autres que le provider LLM configuré

### Accessibilité
- Les raccourcis clavier couvrent au minimum : ouvrir fichier, sauvegarder, undo, redo, basculer onglet Source/Mise en forme, envoyer message Chat, verrouiller sélection
- Le contraste des annotations IA dans l'éditeur doit respecter WCAG AA

### Compatibilité
- Cible principale : Linux x86_64 (cohérent avec le `.goreleaser.yaml` existant)
- Extension macOS et Windows identifiée comme future itération
- Le shell Chromium (lorca) doit être compatible avec Chromium ≥ 90

### Scalabilité
- L'éditeur doit rester fluide sur des documents jusqu'à 50 000 mots
- Le workspace peut contenir jusqu'à 200 fichiers sans dégradation de l'arborescence

---

## Dependencies & Assumptions

### Dépendances techniques

| Dépendance | Rôle | Notes |
|---|---|---|
| `github.com/Bornholm/amatl` | Parsing Markdown, sélecteurs CSS, rendu pipeline complet, génération ToC | Utilisé côté Go backend |
| `github.com/bornholm/genai` | Interface LLM unifiée, agent ReAct, tool calling | Loop agent + streaming |
| Fork de `lorca` | Shell Chromium, pont JS↔Go | Transport des événements UI→backend |
| `goldmark` + extensions GFM | Parsing AST Markdown pour la détection de section active | Déjà utilisé dans amatl |
| Keychain OS | Stockage sécurisé des clés API | À identifier : `zalando/go-keyring` ou équivalent |

### Assumptions
- Le fork de lorca expose un mécanisme de communication bidirectionnel JS↔Go (appels de fonctions exposées + événements push)
- amatl expose une API Go (pas seulement CLI) pour le parsing, la sélection CSS et le rendu — **à confirmer**
- genai supporte le tool calling en mode streaming — visible dans le code fourni, à valider pour tous les providers cibles
- L'utilisateur dispose d'un Chromium installé sur sa machine

### Risques

| Risque | Probabilité | Impact | Mitigation |
|---|---|---|---|
| amatl n'expose pas d'API Go directement utilisable | Moyenne | Élevé | Wrapper CLI avec communication via stdin/stdout, ou refactoring d'amatl pour exposer des packages publics |
| Latence du rendu amatl complet trop élevée | Faible | Moyen | Rendu asynchrone avec cache, rendu partiel de la section active uniquement |
| Conflit de modifications humain/IA simultanées | Moyenne | Élevé | Verrou optimiste sur la section active dès qu'une requête IA est en cours |
| Parsing de section active incorrect sur des structures Markdown complexes | Moyenne | Moyen | Tests de conformité sur des documents réels, fallback sur la ligne courante |

---

## Data Requirements

### Modèles de données principaux

**WorkspaceState**
```
- rootPath: string
- openFiles: []FileTab
- activeFileIndex: int
- settings: AppSettings
```

**FileTab**
```
- path: string          // chemin absolu
- content: string       // contenu courant en mémoire
- isDirty: bool         // modifications non sauvegardées
- history: []HistoryEntry
- historyIndex: int
- activeSection: SectionRef
- sectionLocked: bool
```

**HistoryEntry**
```
- id: string
- timestamp: time.Time
- content: string       // snapshot du document à cet état
- author: enum(human, ai)
- aiContext?: string    // résumé de la demande IA si author=ai
- aiMessageId?: string  // lien vers le message Chat déclencheur
```

**SectionRef**
```
- cssSelector: string   // ex: h2:contains("Introduction")
- startLine: int
- endLine: int
- headingTitle: string  // titre de la section parente
- rawContent: string    // contenu Markdown brut de la section
```

**ChatMessage**
```
- id: string
- role: enum(user, assistant)
- content: string
- timestamp: time.Time
- linkedHistoryEntryId?: string  // si le message a déclenché une modif
- isStreaming: bool
```

**AppSettings**
```
- llm.provider: string
- llm.baseURL: string
- llm.model: string
- llm.maxIterations: int
- llm.maxTokens: int
- llm.reasoningEffort: string
// clé API stockée séparément dans le keychain
```

---

## UI/UX Considerations

### Layout général

```
┌─────────────────────────────────────────────────────────────┐
│  [Menu: Fichier | Workspace | Paramètres]          [⚙]      │
├──────────────────────────────────┬──────────────────────────┤
│  [Onglets fichiers ouverts...]   │                          │
│ ┌────────────────────────────┐   │   COLONNE CHAT           │
│ │ [Source] [Mise en forme]   │   │                          │
│ │                            │   │  Section active :        │
│ │  Éditeur Markdown          │   │  ┌──────────────────┐   │
│ │  avec coloration           │   │  │ ## Introduction  │🔒  │
│ │  syntaxique                │   │  └──────────────────┘   │
│ │                            │   │                          │
│ │  [section active]          │   │  [historique messages]   │
│ │  ← surlignage gouttière    │   │                          │
│ │                            │   │  ┌──────────────────┐   │
│ └────────────────────────────┘   │  │ Saisie message   │   │
│                                  │  │            [►]   │   │
│                                  │  └──────────────────┘   │
└──────────────────────────────────┴──────────────────────────┘
```

### Éléments clés

- **Annotations IA dans l'historique** : dans la gouttière de l'éditeur, une icône ✦ (ou couleur distincte) signale les lignes issues de la dernière modification IA (similaire aux annotations de blame Git)
- **Indicateur de section active** : bandeau discret en haut de la colonne Chat, cliquable pour verrouiller/déverrouiller (icône 🔒)
- **Streaming Chat** : curseur clignotant en fin de message pendant que l'agent répond
- **Badge "IA" dans l'historique** : accessible via le menu contextuel de l'éditeur (clic droit → "Voir l'historique") avec filtrage possible humain/IA
- **État de chargement du rendu** : l'onglet "Mise en forme" affiche un spinner pendant le rendu amatl, avec le message d'erreur amatl si le rendu échoue
- **Fichiers non sauvegardés** : point orange sur l'onglet du fichier (convention standard)

### Responsive / Redimensionnement
- Les deux colonnes doivent être redimensionnables par drag de la séparation
- Largeur minimale de chaque colonne : 300px

---

## Out of Scope (v1)

- **Collaboration multi-utilisateurs** en temps réel (type CRDTs)
- **Panneau d'arborescence** du workspace (navigation fichiers) — prévu v2
- **Diff visuel** ligne à ligne entre avant/après modification IA (prévu v2 — utile mais complexe)
- **Support macOS et Windows** — v2
- **Export direct** depuis l'éditeur (PDF, HTML) — l'utilisateur utilise la CLI amatl
- **Gestion de version Git** intégrée
- **Mode hors-ligne** avec modèle local (Yzma) — architecture prévue pour le supporter, activation v2
- **Historique de conversations** persisté entre sessions
- **Thèmes** de l'éditeur (dark mode, etc.)