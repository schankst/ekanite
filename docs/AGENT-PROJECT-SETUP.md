# Projektsetup für die agentische Umsetzung (Wellenmodell)

Dieses Dokument beschreibt das Setup und die CI-Integration für die Agenten aus `docs/AGENTS.md` sowie die Empfehlung konkreter KI-Agenten-Werkzeuge.

---

## 1. Repo-Layout (Erweiterungen)

```
.github/
  workflows/
    agent-gates.yml        # Gates G0–G3 (CI-Check)
  pull_request_template.md  # DoD-Checkliste für Agenten-PRs
docs/
  AGENTS.md                 # Agentenbeschreibungen (normativ)
  ROADMAP.md                # Wellenmodell, Graph, Gates, Eigentum
  SUPPLY-CHAIN.md           # (geplant) Update-Politik (#83)
  ARCHITECTURE.md           # (geplant) Maintainer-Referenz (#79)
deploy/
  systemd/ekanited.service  # (geplant) OPS-4
```

## 2. Setup-Schritte

1. **Labels** anlegen: `status:in-progress`, `status:blocked`, `status:done-review` (TaskManager-Pflege).
2. **Branch-Schutz** auf `master`: **aktiviert** (2026-11-07, nach Veröffentlichung des Repos): 1 Review (Mensch), Checks `Gate G0 — fmt/vet/build/test` + `Gate — Issue/Milestone-Konsistenz`, Strict-Branch, lineare Historie, enforce_admins.
3. **Env für TaskManager/CI:** `GH_TOKEN` (Repo-Scope) als Secret `repo-token` hinterlegen.
4. **PR-Template** aktivieren (siehe §5).
5. **Milestones** existieren bereits (MS1–MS7) und sind die Wave-Laufräume.

---

## 3. CI: `agent-gates.yml` (Gates)

Jobs und ihre Zuordnung zu Gates:

| Job | Gate | Blockierend? | Anmerkung |
|---|---|---|---|
| `baseline` (fmt/vet/build/test/mod verify) | G0 | ✅ | ab W0 sofort aktiv |
| `race` | G1 | ⚠️ `continue-on-error` bis MS2 | bekannt rot: bbolt checkptr (#3) |
| `vuln` | G1 | ⚠️ `continue-on-error` bis SC-1 | `govulncheck` |
| `lint` | G0/G2 | ⚠️ `continue-on-error` bis #84 | golangci-lint |
| `coverage` | G2 | ✅ ab Implementierung (Baseline 50,9 %) | Gesamtwert darf nicht sinken |
| `hygiene` | konstant | ✅ | alle offenen Issues haben Milestone |

Begründung `continue-on-error`: Der Ist-Zustand ist rot (Race-Gate #3, vuln). Die Gates werden **sichtbar** geführt, blockieren aber erst nach Erreichen ihres Milestones (G1/G2). Das wird mit der Freischaltung in #39/#82 auf `continue-on-error: false` umgestellt.

## 4. KI-Agenten — Empfehlung

### 4.1 Vergleich (für dieses Repo: Go-Standardlib, GitHub-zentrisch, 89 Issues)

| Werkzeug | Stärke | Schwäche für dieses Setup | Einsatz |
|---|---|---|---|
| **Claude Code** (Anthropic) | Terminal-basiert, gut in großen Codebasen, Subagenten-Modell | Lizenzkosten; projektspezifische Konventionen müssen in `AGENTS.md` stehen | **Empfohlen als Haupt-Worker** (Ingest/Engine/Server/Ops) |
| **OpenAI Codex CLI** | Integriert mit GitHub/Issues, Sandbox, Skript-Execution | jünger; Steuerung über Custom-Agents noch im Fluss | gute Alternative für Worker; ideal für TaskManager-Skripte |
| **GitHub Copilot coding agent** | Native GitHub-Integration, PR-Erstellung | weniger flexibel bei Cross-Repo-/Multi-Agent-Orchestrierung | für kleine Tasks + Docs/Testing |
| **Devin** (Cognition) | autonome End-to-End-Tasks | für einen privaten Fork überdimensioniert; Kosten | nicht empfohlen |
| **OpenHands** (open source) | selbst gehostet, frei, Workspace-Steuerung | Setup-Pflege; Qualität kontextabhängig | Option für Selbsthosting |
| **AutoGen / CrewAI / LangGraph** | echte Multi-Agent-Frameworks | zusätzliche Infrastruktur; für 8 Rollen + deterministisches Wellenmodell unnötig | nicht empfohlen |

### 4.2 Empfohlene Konfiguration
- **Supervisor:** Mensch + LLM-Assistenz (z. B. Claude Code im „Read-only/Plan“-Modus) — entscheidet, schreibt keinen Produktionscode.
- **TaskManager:** deterministisches Skript (Bash + `gh` CLI) statt LLM — Planung ist Datenfluss, keine Kreativität. Optional: Codex-CLI-Agent, der nur `gh`-Aufrufe ausführt.
- **Worker (8 Rollen):** 1–2 instanziierte Claude-Code-Agenten **pro Lane**, gestartet mit Kontext (Eigentumsliste + DoD aus `docs/AGENTS.md`); Test-first per `GOFLAGS` und CI-Gates.
- **TestingAgent/DocsAgent:** können derselbe Claude-Code-Agent im jeweiligen Eigentumsbereich sein (eigene Session).

### 4.3 Verifizierter lokaler Werkzeug-Stand (2026-11-07)
| Werkzeug | Lokal gefunden | Verdict |
|---|---|---|
| `claude` (Claude Code, /usr/bin/claude) | ✅ | **Primär-Worker** — ausgereift, Subagenten, gut für Go-Multi-Datei-Aufgaben |
| `opencode` (~/.opencode/bin) | ✅ | OSS-Alternative für Worker; Fallback, wenn Modellkosten/Wahlfreiheit zählen |
| `cline` (v3.0.74) | ✅ | nur bei IDE-Arbeit (VS Code); kein Headless-Betrieb |
| `pi` (@earendil-works/pi-coding-agent v1.5.0) | ✅ | Pilot/Kandidat für einzelne Lanes; noch nicht Battle-tested |
| `omp` (lokale Umgebung) | ✅ | Assistenz für Supervisor/TaskManager (Orchestrierung/Doku), kein Worker-Executor |
| `hermes` | ❌ (kein Binary) | aus der Auswahl entfernen |

**Finale Empfehlung:** Worker = **Claude Code** (primär) / OpenCode (Fallback); TaskManager+Supervisor-Assist = **omp** + deterministische `gh`-Skripte; `cline` nur in IDE; `pi` als Pilot für 1 Lane evaluieren; `hermes` entfällt.

**Warum diese Wahl:** Das Wellenmodell ist deterministisch (Graph + Gates) — der Bedarf ist starke, fokussierte Implementierungs-Agenten (nicht autonome Multi-Agent-Frameworks). Der größte Hebel ist einheitlicher Kontext (`AGENTS.md`), externe Gate-Erzwingung (CI) und ein menschlicher Supervisor. Claude Code + Codex CLI decken das mit minimaler Infrastruktur ab.

## 5. PR-Template (`.github/pull_request_template.md`)

```markdown
## Task
- Issue-Nr.: #<nummer>
- Lane/Wellenmodell: <Lane> / Wave <0-3>
- Eigentumszone: <Dateien>

## DoD-Checkliste
- [ ] Tests geschrieben (Test-first) und `go test ./...` grün
- [ ] `gofmt -l .` leer; `go vet ./...` grün
- [ ] Keine Änderung außerhalb der Eigentumszone
- [ ] Issue-Referenz im Commit
- [ ] (falls relevant) Doku/CHANGELOG aktualisiert

## Gate-Status
- CI agent-gates: <Link>
```
