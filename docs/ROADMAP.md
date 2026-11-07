# Ekanite – Refactoring-Roadmap

Stand: 2026-11-07 · Gültig für den Fork `schankst/ekanite`.
Diese Roadmap ordnet die offenen Issues (21) Milestones zu. Jede Phase hat ein messbares Ziel (Definition of Done). Reihenfolge folgt Abhängigkeiten: nichts ist in einer späteren Phase „doppelt“ anzufassen als in der früheren.

## Sequenz

```
MS1 Baseline ─▶ MS2 Dependencies/CI ─▶ MS3 Korrektheit ─▶ MS4 RFC/Ingest ─▶ MS5 Suche/API
                 │                       ▲
                 └── parallel möglich: MS6 Modularität (niedriges Risiko, kein Pfad-Konflikt) ┘
```

- **MS1 → MS2** zwingend: Doku/CI-Reinigung zuerst, damit spätere Messungen (Race, Vulnerability) belastbar sind.
- **MS2 → MS3** zwingend: `-race`-Gate muss grün sein, bevor Concurrency-Fixes (R1, R2) verifiziert werden.
- **MS3 → MS4** empfohlen: Parser-Fixes (#16/#17) und Framing (#18) berühren dieselben Dateien (`parser/rfc5424.go`, `input/collector.go`); erst Grammatik/Wahrheit, dann SD und Framing.
- **MS6** kann parallel zu MS3–MS5 laufen (eigene Dateien), sollte aber nach MS1 beginnen.

## Milestones und Issues

| Milestone | Fokus | Issues | Definition of Done |
|---|---|---|---|
| **MS1 – Baseline: Hygiene & Doku** | gofmt, Doku, Kleinigkeiten | #1, #6, #12 | `gofmt -l .` leer; `README.md`/`CONTRIBUTING.md` Go-Modul-konform; expvar-Keys korrekt; keine Debug-Ausgabe in Libraries |
| **MS2 – Dependencies & Build-Gates** | bleve v2, bbolt, CI, Modulpfad, Batch-Default | #2, #3, #19, #20 | `go test -race ./...` grün; bleve v2 + `go.etcd.io/bbolt` v1.4; GitHub Actions mit `govulncheck`/Lint; `go.mod`-Modulpfad = `github.com/schankst/ekanite`; `-batchsize`-Default kalibriert |
| **MS3 – Korrektheit & Stabilität** | Fehlerpfade, Panik-Risiken, Races, DoS | #7, #8, #9, #13, #14, #16, #17 | Keine stillen Fehlerverluste (errChan liefert echte Fehler); `NewIndex`-Existenzcheck korrekt; keine Type-Assertion-Panik; `Engine.Search` ohne Lock-Race; Collector beendet stille Verbindungen; RFC-Grammatik akzeptiert konforme Sender |
| **MS4 – RFC-Konformität & Ingest** | Structured Data, Framing, UDP | #15, #18 | SD-Elemente als eigenes Feld geparst; UDP-Puffer ≥ 2048 mit `udpTruncated`; Framing-Fehlschläge gezählt |
| **MS5 – Suche & API** | Zeitfilter, Paging, context | #21 | `Search(ctx, …)` mit `from/until`, Index-Pruning, Paging statt Hard-10.000, Abbruch möglich |
| **MS6 – Modularität & Refactoring** | toter Code, Duplikate, Struktur | #4, #5, #10, #11 | `query/`, `rfc5424/reader.go`-Stub, `NetstrDelimiter` entfernt oder verdrahtet; ein RFC5424-Parser-Quellstand; gemeinsamer Server-Kern; `main.go` < 200 LOC mit getrennten Verantwortungen |

## Reihenfolge-Empfehlung für die Umsetzung

1. **MS1** (klein, unabhängig, schafft Grün-Gefühl) → 0,5–1 Tag.
2. **MS2** (Freischaltung aller Qualitäts-Gates; #20 erst *nach* v2) → 2–3 Tage.
3. **MS3** (kritischste Korrektheits-Defekte; R1/R2 erst jetzt verifizierbar) → 2–4 Tage.
4. **MS4** (RFC-Lücken; setzt auf MS3-Grammatik auf) → 1–2 Tage.
5. **MS5** (Such-API; setzt MS2 voraus, da bleve-v2-SearchRequest) → 3–5 Tage.
6. **MS6** (parallel ab MS1 möglich; idealerweise vor MS5, damit API-Design nicht auf totem Code aufbaut).

## Prämissen und Risiken

- **#20 (Performance)** ist nur *nach* der bleve-v2-Migration gültig; unter bleve v0.8.1 wäre ein Batch-Default von 5000 eine Regression (vgl. Revival-Assessment §5.2).
- **#13** setzt die Lock-Reparatur (#2? nein: #13 ist Issue-Nr. im Revival, hier #21) — die Lock-Reparatur (#13 = R1) voraus; beide berühren `engine.go`.
- Alle Schätzungen sind Grobwerte für einen einzelnen Entwickler ohne Review-Zyklen.
