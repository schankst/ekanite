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
- **Go**: `go 1.24` + `toolchain go1.24.0` (Issue #85).
- **govulncheck**: läuft derzeit noch als `@latest` (Auto-Toolchain). **Pinning ist gekoppelt an eine Go-Anhebung (#129):** ein fester (`@latest`-freier) Lauf scannt die Stdlib der gepinnten Go-Version. Mit Go 1.24 meldet govulncheck **27 Stdlib-Advisories** (html/template, net/http, crypto/tls/x509, …; Fixes in Go 1.25.x, teils nur 1.26.x) — d. h. die gepinnte Go-Version ist nicht stdlib-vulnerabilitätsfrei. Sobald Go ≥ 1.25/1.26 gesetzt ist (#129), wird govulncheck auf eine feste Version gepinnt.

Beim Anheben einer gepinnten Version: lokal ausführen, Schema-/Verhaltensänderungen prüfen, erst dann den Pin aktualisieren.

## Ausnahme-Regel

Ein blockierendes Gate darf **nur** rot sein, wenn:
1. ein `continue-on-error: true` im Workflow liegt **und**
2. ein Folge-Issue den Fix terminiert **und**
3. die Abweichung im PR-Kommentar begründet ist.

Kein Gate ist aktuell ausgenommen; govulncheck blockiert seit #82. Bekannte, nicht aufgerufene Modul-Advisories (govulncheck meldet sie, `exit 0`):
- **GO-2022-0470** (bleve QueryString): `Fixed: N/A` in bleve v1 — nur durch Bluge-Migration (#118) behebbar; Überwachung läuft, Migration ist geplant.
- **GO-2026-5024** (x/sys, Windows-only): Fix v0.44.0 bräuchte Go ≥ 1.25; Ekanite läuft auf Linux (GOOS=windows nicht im Scope) und pinnt Go 1.24 (#85). Bei Go-Anhebung auf ≥ 1.25 automatisch beheben.

## Abdeckungs-Gate (G2)

- `go test -coverprofile` über `./...`; **Gesamtwert darf durch einen PR nicht sinken** (Baseline 50,9 %, Stand 2026-11-07; liegt aktuell höher).
- Das Coverage-Profil wird als **CI-Artifact** (`coverage.out`) pro PR hochgeladen — der `-cover`-Diff ist damit im PR sichtbar.
- Neue Pakete starten mit **≥ 50 %** Abdeckung (Tests ab #22–#25, Ziel ≥ 85 % Kern in #89).

## Performance-Gate (nach MS2)

- **Geplant** (Block, abhängig von #77 Benchmark-Suite): Benchmark-Smoke für Ingest, keine Regressions-Ergebnisse > 10 %.
- Solange #77 offen ist, gibt es kein Performance-Gate; Performance-Änderungen werden über die Test-Suite + manuelle Smoke-Messung abgesichert.

## Verantwortung

- **Supervisor** (docs/AGENTS.md §1) bewertet DoD + CI-Gates vor der Merge-Genehmigung.
- **Worker** führen Tests/Checks lokal aus; die Erzwingung übernimmt die CI.
- Abweichungen vom Vertrag erfordern ein Issue (dieses Dokument ist dann zu aktualisieren).
