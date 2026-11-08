# Release-Prozess (GoReleaser + GHCR)

*(Issue #138 — Release-Pipeline; kein CI-Gate, sondern Tag-getriggerter Veröffentlichungsworkflow)*

Dieses Dokument beschreibt, wie ein Release des Forks erstellt und verifiziert wird. Die Pipeline nutzt [GoReleaser](https://goreleaser.com/) (`.goreleaser.yaml`) und einen GitHub-Actions-Workflow (`.github/workflows/release.yml`), der bei einem `v*`-Tag ausgelöst wird.

## Voraussetzungen

- Sichtbare Git-Commits seit dem letzten Release (Changelog/Git-Historie klären).
- CI-Gates grün auf `master` (fmt/vet/test/race/vuln/lint/coverage — siehe `docs/QUALITY-GATES.md`).
- GHCR-Write-Permission für `ghcr.io/schankst/ekanite` (automatisch über das Repo-Paket).

## Ablauf

1. Auf `master`: **Tag setzen und pushen**:
   ```bash
   git checkout master && git pull
   git tag v1.4.0
   git push origin v1.4.0
   ```
   Der Workflow `publish-release` läuft automatisch.

2. **Workflow-Ergebnis prüfen**:
   - GitHub-Release `v1.4.0` mit Binaries (`linux/darwin/windows × amd64/arm64/386`), `checksums.txt`, `<Binary>.sbom.json`.
   - GHCR-Image `ghcr.io/schankst/ekanite:1.4.0` + `:latest`.

## Verifikation

- **Checksums**: Die Archive von allen heruntergeladenen Release-Assets gegen `checksums.txt` prüfen.
  ```bash
  sha256sum -c checksums.txt
  ```
- **SBOM**: Das `sbom.json` eines Artefakts öffnen und prüfen, dass es den Ekanite-Modulpfad + bleve/bbolt + die erwartete Version enthält.
- **Binär-Version**:
  ```bash
  ./ekanited -version
  # -> ekanite v1.4.0 (<short-commit>) go1.24.x
  ```
- **Image pullbar**:
  ```bash
  docker pull ghcr.io/schankst/ekanite:1.4.0
  docker run --rm ghcr.io/schankst/ekanite:1.4.0 -version
  ```

## Datadir-Kompatibilität

Für ein Upgrade die Datadir-Kompatibilität der Zielversion prüfen (README §Deployment, `docs/OPERATIONS.md`): Dienst stoppen → Binary/Image ersetzen → starten. Das Datadir bleibt erhalten.

## Hinweise

- Der Workflow baut nur **amd64** als Container-Image (Dockerfile ist amd64-focused, `--platform=linux/amd64`); andere Plattformen stehen als Archive zur Verfügung.
- Ein nicht ordinär-versorier Tag (z. B. `v1.4.0-rc1`) wird von den `v*`-Patterns erfasst und ebenso veröffentlicht — nur mit der entsprechenden Versionsbezeichnung.
- Für private/Fehlversuche: ein Tag lässt sich entfernen (`git tag -d` + `git push origin :refs/tags/…`), das veröffentlichte GHCR-Tag bleibt jedoch bestehen.
