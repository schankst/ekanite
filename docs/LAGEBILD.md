# Lagebild — Ekanite

**Stand:** 2026-11-08 · **Basis:** `master` @ `3107eb3` (PR #124)
**Zweck:** einseitiges, lebendes Status-Doc. Wird bei Anforderung neu generiert — siehe Skill `lagebild` (`~/.agents/skills/lagebild/SKILL.md`).

## Kurzfazit

- **Zwei Milestones fertig:** Baseline (15/15) **und RFC & Ingest (11/11)** — letzterer via #74 (TLS-Reload).
- **Alle 7 CI-Gates grün und blockierend** (inkl. govulncheck seit #82, SBOM-Job neu); Branch-Protection führt 6 required checks.
- **MS2-Hygiene-Batch komplett** (#85 Go 1.24, #39 Gate-Vertrag, #82 Supply-Chain) → Dependencies-MS 5/15; **#129 (Go 1.25)** als Nachfolger angelegt.
- **Dependabot aktiv seit #82** — hat bereits 4 Actions-Upgrade-PRs erzeugt (#125–#128), Checks offen.
- **Nächste Ziele:** MS3-Resilienz-Kern (#51/#49/#46/#13) oder #19 (GoReleaser/Modulpfad) oder #129 (Go 1.25).

## Milestone-Status

| Milestone | offen | zu | Anteil zu | Charakter |
|---|---|---|---|---|
| Baseline: Hygiene & Doku | 0 | 15 | 100 % | ✅ fertig |
| RFC-Konformität & Ingest | 0 | 11 | 100 % | ✅ fertig (#74) |
| Dependencies & Build-Gates | 11 | 5 | 31 % | Hygiene-Batch fertig; offen: #129/#118/#19/#76/#77/#20/#63 |
| Korrektheit & Stabilität | 26 | 5 | 16 % | größter Rückstand, produktionskritisch |
| Modularität & Refactoring | 7 | 4 | 36 % | GO-BPs (#85/#86) zu |
| Suche & API | 9 | 0 | 0 % | Nutzer-Features, unbegonnen |
| Observability & Monitoring | 8 | 0 | 0 % | unbegonnen |

## Gates (CI, `master`)

| Gate | Status | blockierend |
|---|---|---|
| G0 — fmt/vet/build/test | ✅ pass | ja |
| G1 — go test -race | ✅ pass | ja |
| G1 — govulncheck | ✅ pass | ja (#82) |
| G0/G2 — golangci-lint | ✅ pass | ja |
| G2 — Abdeckung nicht fallend | ✅ pass | ja (#39) |
| SBOM & Checksums | ✅ pass | ja (#82, neu) |
| Issue/Milestone-Konsistenz | ✅ pass | ja |
| Branch-Protection | 6 required checks | ja |

## Letzte Arbeit

| PR | Inhalt |
|---|---|
| #124 | Issue #82 — govulncheck blockierend, Dependabot, SBOM-Job, x/sys 2020→0.34 |
| #123 | Issue #39 — Qualitätsgates-Vertrag formell + Gates hart (coverage aktiv, 6 required checks) |
| #122 | Issue #86 — Go-1.24-Features: `interface{}`→`any`, `sort.Sort`→`slices.SortFunc` |
| #121 | Issue #85 — Go 1.13→1.24 + toolchain, `GOFLAGS=-mod=readonly` |
| #120 | Issue #74 — TLS-Reload auf SIGHUP, `-tlscert`/`-tlskey`, RSA/ECDSA/Ed25519, keine mTLS-Pflicht |

## Aktuell offen / nächste Schritte

1. **Dependabot-PRs (#125–#128)** — GitHub-Actions-Upgrades (checkout/upload-artifact/setup-go/golangci-lint-action); Checks prüfen, ggf. merge.
2. **MS3-Resilienz-Kern (#51/#49/#46/#13)** — Panik-crasht-Prozess, geordneter Shutdown, Batcher-Leak, Engine.Search-Lock-Race (produktionskritisch, größter Rückstand).
3. **#19 (GoReleaser/Modulpfad)** — schließt SBOM-Vollversion + Release-Pipeline + GHCR.
4. **#129 (Go 1.25)** — behebt die letzte hebbare Ausnahme GO-2026-5024/x-sys; MS2-Vollausbau.

## Aufgeschoben / Blockiert

- **#118 (Bluge)**: geplant/etappiert; braucht #76 (Datadir-Vertrag) + #77 (Benchmark) als Basis. Bleibt die einzige nicht-aufgerufene Advisory-Quelle (GO-2022-0470/bleve, `Fixed: N/A`).
- **#20 (Performance), #63 (Autotuning)**: ohne #77-Benchmark-Suite nicht absicherbar.
- **#89 (Testabdeckung/Fuzz)**: groß; nach Resilienz-Batch, wenn betroffene Codepfade ohnehin angefasst werden.
- **GO-2026-5024 (x/sys)**: Windows-only; behebbar ab Go 1.25 (#129).
