# Building

*Building Ekanite requires Go 1.26 or later (see `go.mod` / project toolchain).*

```bash
go build ./...
```

To install the `ekanited` binary (this fork):

```bash
go install github.com/schankst/ekanite/cmd/ekanited@latest
```

Version information can be embedded at build time via `-ldflags` (Issue #43), for example:

```bash
go build -ldflags "-X main.version=$(git describe --tags --always --dirty) -X main.commit=$(git rev-parse --short HEAD)" ./cmd/ekanited
```

# Cloning a fork

If you wish to work with a fork of Ekanite (this repository uses the module/fork path `github.com/schankst/ekanite`), clone your fork and work inside the checked-out module directory:

```bash
git clone git@github.com:<your Github username>/ekanite.git
cd ekanite
go build ./...
```

Go modules handle dependencies; a separate `GOPATH` layout is no longer required.

# Entwicklungsprozess

*(Issue #38 — Prozess-Vertrag)*

1. **Commit-Konvention:** Konventionelle Commits (`feat`, `fix`, `docs`, `refactor`, `test`, `chore`, `build`). Jeder `fix`/`feat`-Commit referenziert ein Issue (`#N`) im Betreff oder Body.
2. **Branching:** Arbeit erfolgt auf Feature-Branches. Merge nur per Pull Request auf `master`; `master` bleibt stabil (geschützt).
3. **Changelog-Pflicht:** Jede Verhaltensänderung ergänzt `CHANGELOG.md` im selben PR.

# Test-Pflicht

*(Issue #40 — Test-Vertrag)*

1. **Neue Funktion:** Jede neue Funktion kommt nur mit Tests (Unit und, wo sinnvoll, E2E) nach `master`.
2. **Bugfix:** Jeder Bugfix bringt einen Regressionstest, der vor dem Fix rot ist.
3. **Pfad-Pflicht:** Änderungen an `parser/`, `input/collector.go`, `status/`, `engine.go` müssen die betroffenen Funktionen abdecken.
4. **Determinismus:** Testläufe erfolgen mit `go test -count=1` (deterministisch, kein Sleep/Net-Flakiness).

# Doku-Vertrag

*(Issue #42 — Doku-Vertrag)*

1. **GoDoc-Pflicht:** Jeder exportierte Typ/Funktion/Var/Const hat einen GoDoc-Kommentar. Prüfung lokal via `scripts/godoc-check.sh` (meldet undokumentierte Symbole, Exit 1 bei Verletzungen). Das Skript soll später im CI laufen (#39); hier nur Skript + Dokumentation, keine Workflow-Änderung.
2. **README/Flag-Synchronität:** Features und Flags in `README.md` spiegeln den aktuellen Stand (`-h`-Ausgabe vs. README im Review prüfen).
3. **CHANGELOG-Pflicht:** Jeder PR mit Verhaltensänderung ergänzt `CHANGELOG.md`.
