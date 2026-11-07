# Agentenbeschreibungen — Umsetzung des Wellenmodells

Gültig für die Roadmap `docs/ROADMAP.md` (Wave 0–3, 7 Milestones, 89 Issues). Dieses Dokument ist die normative Beschreibung aller Agenten, ihrer Rechte und ihres Zusammenwirkens (Best Practices agentischer Programmierung).

---

## 0. Grundsätze (agentische Best Practices)

1. **Single Responsibility:** Jeder Agent besitzt klar abgegrenzte Dateien/Issues (Eigentumsmatrix §4.3 der Roadmap). Keine Cross-Zonen-Änderungen ohne TaskManager-Ticket.
2. **Deterministische Orchestrierung:** Der TaskManager folgt ausschließlich dem Dependency-Graph; keine autonome Neuplanung durch Worker.
3. **Verifizierung vor Abschluss:** Tests/Checks laufen **vor** dem Merge; der Supervisor ist die einzige Instanz, die DoD bestätigt. Agenten dürfen sich nicht selbst genehmigen.
4. **Externe Gates als erzwungene Realität:** CI (`.github/workflows/agent-gates.yml`) blockiert Merge, wenn Gates rot sind — nicht die Disziplin einzelner Agenten.
5. **Mensch-in-der-Schleife:** Branch-Schutz, PR-Review (mind. 1 Mensch), kein Auto-Merge für MS2/SC (kritischer Pfad).
6. **Kleinste Kontexte:** Jeder Agent bekommt nur den Task, die DoD und die Eigentumsliste — nicht das gesamte Repo/Issue-Set.
7. **Traceability:** Jede Aktion wird über Issue-Kommentar + Commit-Referenz dokumentiert; `status:*`-Labels pflegt der TaskManager.
8. **Sandbox:** Tests laufen in der CI-Umgebung; kein Agent hat direkten Push auf `master`.

---

## 1. Steuerungs-Agenten

### SupervisorAgent
| Attribut | Wert |
|---|---|
| Zweck | Qualitätssicherung und End-Entscheidung |
| Rechte | Merge-Genehmigung, DoD-Bewertung, Umbrella-Issues (#88/#89/#42), Gate-Prüfung G0–G3 |
| Pflichten | Verifiziert Ergebnisse gegen Definition of Done; gibt Tasks bei Nichterfüllung mit Befund zurück |
| Verboten | Produktionscode-Änderungen selbst vorzunehmen |
| Auslösung | Bei jedem PR auf `master`, jedem Milestone-Abschluss, jeder Umbrella-Bewertung |

### TaskManagerAgent
| Attribut | Wert |
|---|---|
| Zweck | Orchestrierung des Wellenmodells |
| Rechte | Issue-Zustände (`status:in-progress/blocked`), Task-Zuordnung, Deblockieren von Nachfolgern, Eigentumsvergabe |
| Pflichten | Liest Dependency-Graph (Roadmap §2), plant Lanes, konvertiert Issues in Tasks, sammelt Ergebnisse, meldet Fortschritt |
| Verboten | Fachliche Implementierung; Entscheidungen über DoD |
| Auslösung | Kontinuierlich; nach jedem abgeschlossenen Task |

---

## 2. Ausführende Agenten (Worker)

Alle Worker: **Test-first**, arbeiten in Feature-Branches, liefern PRs mit Bezug auf Issue und DoD.

### IngestWorker
- **Zweck:** Parser/Framing/Collector/TLS-Ingest.
- **Eigentum:** `parser/*.go`, `input/collector.go`, `input/delimiter_*.go`, `input/event.go`.
- **Issues:** #15–#18, #33, #22, F5/F7/F8, E2-Ingest-Anteil (#14/#47).
- **DoD:** `go test ./...` grün; neue Tabelle-Tests; keine neuen Panik-Pfade (#51).

### EngineWorker
- **Zweck:** Engine/Index/Suche/Retention/Batcher.
- **Eigentum:** `engine.go`, `index.go`, `event.go`.
- **Issues:** #7/#8/#9/#13/#46/#51/#67, F1/F2, #21, MON-6.
- **DoD:** `-race` grün; Lock-Invarianten eingehalten (Roadmap §2 Kante #13).

### ServerWorker
- **Zweck:** HTTP/Telnet/API/Auth/Timeouts.
- **Eigentum:** `server.go`, `server_http.go`, `status/service.go`.
- **Issues:** RES-3, #37, #48, GAP-2, #78, S1-relevante Schutzmaßnahmen.
- **DoD:** `httptest`-Tests; keine unbounded Ram-Sammlung mehr.

### OpsWorker
- **Zweck:** CLI, Konfiguration, Tuning, Deployment, systemd, Health.
- **Eigentum:** `cmd/ekanited/main.go`, Flags, `deploy/`.
- **Issues:** CLI-1..3, TUN-1..5, OPS-4, GAP-3, #72.
- **DoD:** `-h`-Ausgabe == README-Flagliste; Validate-Table-Tests.

### ObservabilityWorker
- **Zweck:** Metriken, Logging, Probes, OTel.
- **Eigentum:** `metrics/` (neu), `status/`, slog-Integration.
- **Issues:** MON-1..7, OPS-6.
- **DoD:** `/metrics`-Fixture-Tests; slog-JSON-Test.

### SupplyChainWorker
- **Zweck:** Deps, Go-Version, CI, SCM, Vulnerabilities.
- **Eigentum:** `go.mod`, `go.sum`, `.github/workflows/*`.
- **Issues:** MS2 komplett (SC-1..4, #19, #85, #2/#3/#20/#76).
- **DoD:** G1-Gate (Race grün, vuln 0, keine archivierten Module).

### TestingAgent (unterstützend)
- **Zweck:** Test-Infrastruktur, Fuzz, Coverage, Regression-Harness.
- **Eigentum:** `*_test.go`, Benchmark-/Fuzz-Ziele, Coverage-Gate.
- **Issues:** #23–#25, #75, #89, T1–T4.
- **DoD:** Kernpakete ≥ 85 %; Fuzz-5-min-CI-Lauf.

### DocsAgent (unterstützend)
- **Zweck:** README, CHANGELOG, GoDoc, OPERATIONS, ARCHITECTURE.
- **Eigentum:** `docs/*`, `README.md`, `CHANGELOG.md`.
- **Issues:** #1/#26/#27/#28/#42/#79, #88.
- **DoD:** GoDoc-Check 0 Lücken; README/Flags synchron.

---

## 3. Betriebsprotokoll (Task-Lebenszyklus)

1. **TaskManager** wählt nächste Aufgabe aus dem Graph (keine offenen Vorgänger).
2. **TaskManager** erstellt Branch `task/<issue>-<lane>`, setzt Label `status:in-progress`.
3. **Worker/Agen** implementiert test-first; committet mit `#<issue>`-Referenz.
4. **Worker** öffnet PR; CI `agent-gates` läuft.
5. **Supervisor** prüft DoD + CI-Gates; bei Mängeln → zurück an Worker.
6. Merge durch Supervisor (Branch-Schutz); TaskManager schließt Issue, deblockiert Nachfolger.

---

## 4. Gegenprüfung
- Roadmap §2 (Graph), §3 (Lanes), §4.3 (Eigentumsmatrix), §5 (Gates).
- CI: `.github/workflows/agent-gates.yml`.
