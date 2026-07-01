# Procédure de packaging Amatled pour Manjaro / Arch Linux

Ce document décrit comment générer et installer un paquet Pacman (`.pkg.tar.zst`) pour Amatled, avec icône et lanceur graphique.

---

## Prérequis

Sur une machine Manjaro ou Arch Linux, assurez-vous d’avoir les outils suivants :

```bash
sudo pacman -Syu
sudo pacman -S go nodejs npm webkit2gtk-4.1 gtk3 base-devel imagemagick
```

> `go`, `webkit2gtk-4.1` et `gtk3` sont nécessaires car le binaire est compilé avec `CGO_ENABLED=1`.

### Installer GoReleaser

GoReleaser n’est pas forcément dans les dépôts officiels. Vous pouvez l’installer via le binaire release :

```bash
curl -sL -o /tmp/goreleaser.tar.gz \
  https://github.com/goreleaser/goreleaser/releases/download/v2.14.0/goreleaser_Linux_x86_64.tar.gz
tar -xzf /tmp/goreleaser.tar.gz -C /tmp goreleaser
install -Dm755 /tmp/goreleaser ~/.local/bin/goreleaser
```

Vérifiez l’installation :

```bash
goreleaser --version
```

---

## Générer le paquet Pacman

Depuis la racine du dépôt :

```bash
cd /chemin/vers/amatled
rm -rf dist go
goreleaser release --snapshot --clean
```

Après le build, les artefacts sont dans `./dist/` :

- `amatled_*.pkg.tar.zst` — paquet Pacman
- `amatled_*.tar.gz` — archive binaire Linux
- `amatled_*.zip` — archive binaire Windows
- `checksums.txt` — sommes de contrôle

Vous pouvez aussi utiliser le raccourci Makefile :

```bash
make package
```

> `make package` nettoie `dist/` et `go/` puis lance `goreleaser release --snapshot --clean` directement.

---

## Installer le paquet

```bash
sudo pacman -U dist/amatled_*.pkg.tar.zst
```

Puis mettez à jour les bases de données d’icônes et de fichiers `.desktop` :

```bash
sudo update-desktop-database /usr/share/applications
sudo gtk-update-icon-cache /usr/share/icons/hicolor
```

Il peut être nécessaire de fermer/ré-ouvrir la session graphique pour que l’icône dans la barre des tâches soit prise en compte.

---

## Vérifier le résultat

### Double-clic / clic droit

- Double-clic sur l’icône Amatled : lance l’application.
- Clic droit sur un dossier → **Ouvrir avec Amatled** : ouvre le dossier comme workspace.
- Clic droit sur un fichier `.md` → **Ouvrir avec Amatled** : ouvre le dossier parent et active le fichier.

### En ligne de commande

```bash
amatled /chemin/vers/workspace
amatled /chemin/vers/document.md
```

---

## Désinstaller

```bash
sudo pacman -R amatled
```

---

## Contenu du paquet

Le paquet installe :

- `/usr/bin/amatled` — binaire
- `/usr/share/applications/amatled.desktop` — lanceur
- `/usr/share/mime/packages/amatled.xml` — association MIME

Icônes :

- `/usr/share/icons/hicolor/scalable/apps/amatled.svg`

---

## Packaging manuel (sans GoReleaser)

Si vous préférez ne pas utiliser GoReleaser, vous pouvez compiler puis installer localement les fichiers :

```bash
make build
make install-desktop
update-desktop-database ~/.local/share/applications/
```

Cela installe le binaire et le raccourci dans votre profil utilisateur (`~/.local/`), sans créer de paquet distribuable.

---

## Fichiers concernés

- `.goreleaser.yaml` — configuration GoReleaser (build + nfpm Pacman)
- `internal/app/app.go` — création de la fenêtre lorca avec `--class=amatled`
- `misc/packaging/linux/amatled.desktop` — lanceur + `StartupWMClass`
- `misc/packaging/linux/amatled.xml` — association MIME
- `misc/packaging/icons/` — icônes PNG
- `Makefile` — cibles `package` et `install-desktop`
