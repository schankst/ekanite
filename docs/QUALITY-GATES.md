# Qualitätsgates — Vertrag (C2, Issue #39)

**Zweck:** Festschreiben, welche automatisierten Gates **vor** einem Merge auf `master` grün sein müssen. Qualität ist nicht verhandelbar; der Branch-Schutz erzwingt die Gates technisch, nicht die Disziplin einzelner Agenten.

**Quelle der Wahrheit fürs Detail:** `.github/workflows/agent-gates.yml` (Jobs, Schritte, `continue-on-error`). Dieses Dokument hält die normativen Regeln fest; der Workflow ist die Implementierung.

## Pflicht-Gates (blockierend, vor jedem Merge)

| Gate | Check (Job `agent-gates`) | Blockierend seit |
|---|---|---|
| G0 | `gofmt -l .` leer · `go vet ./...` · `go build ./...` · `go test -count=1 ./...` · `go mod verify` | W0 (#106) |
| G1 | `go test -race -count=1 ./...` | #117 (bleve v1.0.14) |
| G1 | `govulncheck ./...` (0 aufgerufene Vulnerabilities) | #82 |
| G0/G2 | golangci-lint v2 (gofmt-govet/errcheck/staticcheck/unused; gosimple ⊂ staticcheck) | #116 |
| G2 | Abdeckung nicht fallend (Gesamtwert ≥ Baseline, s. u.) | #39 |
| SBOM | SBOM (`go version -m`) + sha256-Checksums als CI-Artifact `sbom` | #82 |
| Hygiene | alle offenen Issues haben einen Milestone | #108 |

**Erzwingung:** Alle blockierenden Gates sind als **required status checks** in der Branch-Protection von `master` registriert. Ein PR mit rotem Gate ist **nicht mergebar** — kein „Known-Failure-Schlupf" (Issue #3).

## Tool- und Action-Versionen (Pinning)

Die Gate-Toolchain wird **gepinnt**, damit ein Upstream-Release nicht unangekündigt ein Gate bricht (Issue #142; ausgelöst durch #125/#126):

- **GitHub-Actions**: Major-Pin (`@vN`), gepflegt via Dependabot (`.github/dependabot.yml`).
- **golangci-lint**: Tool-Version fest im Workflow (`version: v2.15.0`), nicht `latest` → kein Sprung auf ein inkompatibles Major/Schema.
- **Go**: `go 1.26` + `toolchain go1.26.10` (Issue #129; zuvor #85 mit 1.24). Der Sprung war nötig, weil ein gepinnter govulncheck-Lauf die Stdlib der gepinnten Go-Version scannt und Go 1.24 **27 Stdlib-Advisories** aufwies (Fixes erst in Go 1.25.x/1.26.x).
- **govulncheck**: feste Version (`golang.org/x/vuln/cmd/govulncheck@v1.9.0`), nicht `@latest` → läuft unter dem gepinnten Go 1.26 und scannt dessen Stdlib; kein Auto-Toolchain-Download.
- **goreleaser**: feste Tool-Version (`version: v2.20.0` in `release.yml`), nicht `latest` → reproduzierbare Releases (#138).

Beim Anheben einer gepinnten Version: lokal ausführen, Schema-/Verhaltensänderungen prüfen, erst dann den Pin aktualisieren.

## Ausnahme-Regel

Ein blockierendes Gate darf **nur** rot sein, wenn:
1. ein `continue-on-error: true` im Workflow liegt **und**
2. ein Folge-Issue den Fix terminiert **und**
3. die Abweichung im PR-Kommentar begründet ist.

Kein Gate ist aktuell ausgenommen; govulncheck blockiert seit #82. Bekannte, nicht aufgerufene Modul-Advisories (govulncheck meldet sie, `exit 0`):
- **GO-2022-0470** (bleve QueryString): `Fixed: N/A` in bleve v1 — nur durch Bluge-Migration (#118) behebbar; Überwachung läuft, Migration ist geplant.
- *(GO-2026-5024/x-sys ist mit dem Go-1.26-Sprung und x/sys v0.50.0 entfallen — nicht mehr gemeldet.)*

## Abdeckungs-Gate (G2)

- `go test -coverprofile` über `./...`; **Gesamtwert darf durch einen PR nicht sinken** (Baseline 50,9 %, Stand 2026-11-07; liegt aktuell höher).
- Das Coverage-Profil wird als **CI-Artifact** (`coverage.out`) pro PR hochgeladen — der `-cover`-Diff ist damit im PR sichtbar.
- Neue Pakete starten mit **≥ 50 %** Abdeckung (Tests ab #22–#25, Ziel ≥ 85 % Kern in #89).

## Komplexitäts-Report (informativ)

- Der CI-Job **`Komplexität (gocyclo, informativ)`** gibt bei jedem Lauf die **Top-10-Funktionen nach zyklomatischer Komplexität** (und den Durchschnitt) aus — **nicht blockierend** (`continue-on-error: true`, Issue #87).
- Ziel: Sichtbarkeit statt Gate. Top-Funktionen sind entweder **vereinfacht** (z. B. `assert` entfernt #84, `DocIDs.Less`→`sortDocID` #86, Collector-Kaskaden #14, `main` entflochten #11/#87) **oder mit Tests belegt** (Query-Server `handleConnection`/`ServeHTTP` getestet, #87).
- Beim Hinzufügen hochkomplexer Funktionen: prüfen, ob sie aufteilbar sind; sonst mit Tests absichern.

## Performance-Gate (nach MS2, #77)

- **Benchmark-Suite:** `bench_test.go` (Engine/Batch-Indexierung) und `input/bench_test.go` (Framing/Parser). CI-Job **`Benchmark (informativ)`** — gebunden (`-benchtime=1x`), Artefakt `benchmarks`, **nicht blockierend** (`continue-on-error: true`).
- **Richtwerte:** `docs/CAPACITY.md` (Events/s, Disk/Event, RAM, Annahmen).
- **Regressionsregel:** ein Ergebnis **> 10 %** schlechter als die Referenz ist ein Review-Blocker (#39). Der belastbare Vergleich erfolgt per `benchstat` (Rezept in `docs/CAPACITY.md`); ein automatisch blockierendes Zeit-Gate ist auf geteilten CI-Runnern zu verrauscht und daher bewusst nicht aktiviert.

## Verantwortung

- **Supervisor** (docs/AGENTS.md §1) bewertet DoD + CI-Gates vor der Merge-Genehmigung.
- **Worker** führen Tests/Checks lokal aus; die Erzwingung übernimmt die CI.
- Abweichungen vom Vertrag erfordern ein Issue (dieses Dokument ist dann zu aktualisieren).
