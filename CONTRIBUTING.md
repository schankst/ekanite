# Building
*Building Ekanite requires Go 1.9 or later. [gvm](https://github.com/moovweb/gvm) is a great tool for installing and managing your versions of Go.*

Tested on 64-bit Kubuntu 14.04.

    mkdir ~/ekanite # Or a directory of your choice.
    cd ~/ekanite
    export GOPATH=$PWD
    go get github.com/ekanite/ekanite
    go install github.com/ekanite/...

# Cloning a fork
If you wish to work with a fork of Ekanite, your own fork for example, you must still follow the directory structure above. But instead of cloning the main repo, instead clone your fork. You must fork the project if you want to contribute upstream.

Follow the steps below to work with a fork:

```bash
    export GOPATH=$HOME/ekanite
    mkdir -p $GOPATH/src/github.com/ekanite
    cd $GOPATH/src/github.com/ekanite
    git clone git@github.com:<your Github username>/ekanite
```

Retaining the directory structure `$GOPATH/src/github.com/ekanite` is necessary so that Go imports work correctly.

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
