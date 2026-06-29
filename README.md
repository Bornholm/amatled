<p align="center">
  <img src="./misc/resources/logo.svg" style="height:150px" />
</p>

# `amatled`

Éditeur Markdown desktop assisté par IA, basé sur [amatl](https://github.com/Bornholm/amatl) et [genai](https://github.com/bornholm/genai). Transformez vos documents Markdown avec un assistant IA capable de comprendre et modifier la structure de votre document.

> **Pourquoi le nom `amatled` ?**
>
> AmatlEd (de Amatl + Editor) fait directement référence à son grand frère [amatl](https://github.com/Bornholm/amatl) — l'outil CLI de transformation de documents Markdown — dont il reprend le nom et le cœur du pipeline de rendu. Le suffixe "Ed" évoque l'édition interactive et la dimension desktop de l'application.
>
> Comme amatl tire son nom du papier d'écorce préhispanique utilisé pour créer des codex, AmatlEd est le scribe qui vous assiste dans la rédaction de vos propres codex numériques.

## Fonctionnalités

- Éditez vos fichiers Markdown dans un environnement desktop natif avec coloration syntaxique (CodeMirror 6) ;
- Basculez entre vue **Source** et **Rendu** (pipeline amatl complet : MermaidJS, directives, tables des matières) ;
- Collaborez avec un **assistant IA** capable de lire et modifier la section active de votre document ;
- Bénéficiez d'un **historique unifié** couvrant modifications humaines et IA, avec undo/redo et rollback par message ;
- Gérez vos **profils LLM** (OpenAI, OpenRouter, Mistral, compatible OpenAI) avec stockage sécurisé des clés via le keychain système ;
- Travaillez sur des **workspaces multi-fichiers** avec arborescence, onglets, et résolution des inclusions amatl.

## Installation

- [package_doc](./package.md)

### Arch Linux / Manjaro

Un paquet Pacman (`.pkg.tar.zst`) est disponible dans les [releases GitHub](https://github.com/Bornholm/amatled/releases/latest). Téléchargez-le puis installez-le avec :

```shell
sudo pacman -U amatled_<version>_linux_amd64.pkg.tar.zst
```

Amatled apparaîtra alors dans le menu applications et pourra être invoqué depuis le terminal :

```shell
amatled /chemin/vers/workspace        # ouvrir un dossier
amatled /chemin/vers/document.md      # ouvrir un fichier Markdown
```

### Depuis les sources

1. Clonez le dépôt et exécutez `make build`
2. Lancez l'éditeur :

```shell
./bin/amatled /chemin/vers/votre/workspace
```

## Utilisation

> **Attention ⚠**
>
> Ce projet est en phase initiale et sujet à une évolution rapide. Attendez-vous à des changements fréquents et à une potentielle instabilité.

1. Téléchargez [la dernière version](https://github.com/Bornholm/amatled/releases/latest) d'`amatled`
2. Dans votre terminal, lancez l'éditeur :

```shell
amatled /chemin/vers/votre/workspace
```

3. Configurez votre provider LLM via le panneau des paramètres (⚙)
4. Ouvrez un fichier Markdown et commencez à éditer avec l'assistant IA

## Licence

[MIT](./LICENCE)
