# Lagebild — Ekanite

**Stand:** 2026-11-08 · **Basis:** `master` @ `2514143` (PR #117)
**Zweck:** einseitiges, lebendes Status-Doc. Wird bei Anforderung neu generiert — siehe Skill `lagebild` (`~/.agents/skills/lagebild/SKILL.md`).

## Kurzfazit

- **Baseline abgeschlossen** (MS1 15/15); **RFC-MS fast fertig** (10/11, nur #74 TLS-Rotation offen).
- **CI-Gates vollständig grün und blockierend**: Lint (#116), `-race` (#2/#3 via bleve v1.0.14 + bbolt 1.3.5), govulncheck, Abdeckung.
- **Nächste Ziele:** #74 (RFC-MS schließen) → MS2-Hygiene (#85 Go-Version, #39 Gate-Vertrag, #82 Dependabot) → MS3-Resilienz-Kern (#51/#49/#46/#13).
- **Bewusst aufgeschoben:** #118 (Bluge-Migration, geplant/etappiert), #20/#63 (Performance, brauchen #77-Benchmark).

## Milestone-Status

| Milestone | offen | zu | Anteil zu | Charakter |
|---|---|---|---|---|
| Baseline: Hygiene & Doku | 0 | 15 | 100 % | ✅ fertig |
| RFC-Konformität & Ingest | 1 | 10 | 91 % | ein Issue von fertig (#74) |
| Dependencies & Build-Gates | 13 | 2 | 13 % | v.a. Hygiene + Bluge (#118) |
| Korrektheit & Stabilität | 26 | 5 | 16 % | größter Rückstand, produktionskritisch |
| Modularität & Refactoring | 8 | 3 | 27 % | techn. Schulden |
| Suche & API | 9 | 0 | 0 % | Nutzer-Features, unbegonnen |
| Observability & Monitoring | 8 | 0 | 0 % | unbegonnen |

## Gates (CI, `master`)

| Gate | Status | blockierend |
|---|---|---|
| G0 — fmt/vet/build/test | ✅ pass | ja |
| G0/G2 — golangci-lint | ✅ pass | ja |
| G1 — go test -race | ✅ pass | ja |
| G1 — govulncheck | ✅ pass | nein (#82) |
| G2 — Abdeckung nicht fallend | ✅ pass | ja |
| Issue/Milestone-Konsistenz | ✅ pass | ja |

## Letzte Arbeit

| PR | Inhalt |
|---|---|
| #117 | Issues #2+#3 — bleve v1.0.14 (Stack 2019→aktuell), bbolt-Pfad repariert, Parser-Stats-Race gefixt, `-race`-Gate blockierend |
| #116 | Issue #84 — Lint-Gate blockierend + Idiomatisches-Go-Audit (49 Findings behoben) |
| #115 | Issues #93+#95 — Feld-Anonymisierung (inet/hmac) + Datensparsamkeit (`-raw drop`) |

## Aktuell offen / nächste Schritte

1. **#74 (TLS-Rotation)** → RFC-MS auf 11/11 schließen. `main.go:236-273`: nur PKCS#1-RSA, einmalig geladen, kein Reload → Reload-Pfad + ECDSA/Ed25519 + Chain.
2. **MS2-Hygiene-Batch (#85 + #39 + #82)** — go.mod `go 1.13`→aktiv gültig, Gate-Vertrag formell, Dependabot/govulncheck.
3. **MS3-Resilienz-Kern (#51/#49/#46/#13)** — Panik-crasht-Prozess, geordneter Shutdown, Batcher-Leak, Engine.Search-Lock-Race.
4. Danach: **Suche & API (MS5)** — #37 JSON-API, #29/#30 Zeitfilter/Paging (nach stabiler Engine).

## Aufgeschoben / Blockiert

- **#118 (Bluge)**: geplant/etappiert; braucht #76 (Datadir-Vertrag) + #77 (Benchmark) als Basis.
- **#20 (Performance), #63 (Autotuning)**: ohne #77-Benchmark-Suite nicht absicherbar.
- **#89 (Testabdeckung/Fuzz)**: groß; nach Resilienz-Batch, wenn betroffene Codepfade ohnehin angefasst werden.
