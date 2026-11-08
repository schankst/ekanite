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
2. **Branch-Schutz** auf `master`: **aktiviert** (2026-11-07, nach Veröffentlichung des Repos): Checks `Gate G0 — fmt/vet/build/test` + `Gate — Issue/Milestone-Konsistenz`, Strict-Branch, lineare Historie, enforce_admins. **Review-Pflicht = 0** (Solo-Fork: `required_approving_review_count=0`); ein Menschlicher Review ist fehlblockiert, weil nur ein Contributor existiert („can’t approve your own PR“). Wird auf 1 erhöht, sobald ein zweiter Reviewer/Bot vorhanden ist.
3. **Env für TaskManager/CI:** `GH_TOKEN` (Repo-Scope) als Secret `repo-token` hinterlegen.
4. **PR-Template** aktivieren (siehe §5).
5. **Milestones** existieren bereits (MS1–MS7) und sind die Wave-Laufräume.

---

## 3. CI: `agent-gates.yml` (Gates)

Jobs und ihre Zuordnung zu Gates (verbindliche Regeln: `docs/QUALITY-GATES.md`):

| Job | Gate | Blockierend? | Anmerkung |
|---|---|---|---|
| `baseline` (fmt/vet/build/test/mod verify) | G0 | ✅ | seit W0 aktiv |
| `race` | G1 | ✅ | seit #117 (bleve v1.0.14, bbolt 1.3.5) |
| `vuln` | G1 | ⚠️ `continue-on-error: true` | `govulncheck`; Ausnahme bis #82 (Folge-Issue, s. Vertrag) |
| `lint` | G0/G2 | ✅ | seit #116 (49 Findings behoben) |
| `coverage` | G2 | ✅ | aktiv seit #39; Baseline 50,9 %, Artifact `coverage` |
| `hygiene` | konstant | ✅ | alle offenen Issues haben Milestone |

Alle blockierenden Gates sind als **required status checks** in der Branch-Protection von `master` registriert (G0, G1-race, G0/G2-lint, G2-coverage, Hygiene). Nur die dokumentierte Ausnahme (vuln, #82) läuft sichtbar ohne Blockade.

## 4. KI-Agenten — Empfehlung

### 4.1 Vergleich (für dieses Repo: Go-Standardlib, GitHub-zentrisch, 89 Issues)

| Werkzeug | Stärke | Schwäche für dieses Setup | Einsatz |
|---|---|---|---|
| **OpenCode** | OSS, CLI-first, provider-flexibel (OpenRouter/Venice), gute Modell-Pinning-Optionen | Konfiguration pro Provider nötig | **Primär-Worker** (nach Modell-Festlegung) |
| **pi (pi-coding-agent)** | leichtgewichtig, Coding-Fokus | jung; Modellauswahl pro Task festlegen | Pilot für 1 Lane |
| **Cline** | IDE-Integration (VS Code) | kein Headless; Modellwahl in UI | nur bei IDE-Arbeit |
| **omp** | aktuelle Umgebung/Assistent | nicht als Worker-Executor gedacht | TaskManager/Supervisor-Assist |
| **Claude Code** | (auf Wunsch entfernt) | — | nicht mehr in Auswahl |
| **hermes** | nicht installiert | — | entfällt |

### 4.2 Empfohlene Konfiguration
- **Supervisor:** Mensch + LLM-Assistenz (z. B. omp/OpenCode im „Read-only/Plan“-Modus) — entscheidet, schreibt keinen Produktionscode.
- **TaskManager:** deterministisches Skript (Bash + `gh` CLI) statt LLM — Planung ist Datenfluss, keine Kreativität. Optional: Codex-CLI-Agent, der nur `gh`-Aufrufe ausführt.
- **Worker (8 Rollen):** 1–2 instanziierte **OpenCode**-Agenten **pro Lane**, gestartet mit Kontext (Eigentumsliste + DoD aus `docs/AGENTS.md`); Modell-Pins je Rolle (siehe 4.3); Test-first per `GOFLAGS` und CI-Gates.
- **TestingAgent/DocsAgent:** können derselbe OpenCode-Agent im jeweiligen Eigentumsbereich sein (eigene Session).

### 4.3 Verifizierter lokaler Werkzeug-Stand (2026-11-07)
| Werkzeug | Lokal gefunden | Verdict |
|---|---|---|
| `opencode` (~/.opencode/bin) | ✅ | **Primär-Worker** — CLI-first, Provider OpenRouter/Venice, Modelle per Konfiguration pinbar |
| `pi` (@earendil-works/pi-coding-agent v1.5.0) | ✅ | Pilot/Kandidat für einzelne Lanes |
| `cline` (v3.0.74) | ✅ | nur bei IDE-Arbeit (VS Code); kein Headless-Betrieb |
| `omp` (lokale Umgebung) | ✅ | Assistenz für Supervisor/TaskManager (Orchestrierung/Doku), kein Worker-Executor |
| `claude` | ✅ installiert, **bewusst aus der Auswahl genommen** | — |
| `hermes` | ❌ (kein Binary) | aus der Auswahl entfernen |

**Finale Empfehlung (Werkzeuge):** Worker = **OpenCode** (primär) mit pro-Rolle gepinnten Modellen (siehe Modell-Entscheidung); `pi` als Pilot für 1 Lane; `cline` nur in IDE; TaskManager+Supervisor-Assist = **omp** + deterministische `gh`-Skripte; `claude`/`hermes` entfallen.

**Modell-Entscheidung** (Provider: OpenRouter, Venice) wird in `docs/MODEL-POLICY.md` geführt (geplant); Pins erst nach Verifikation der verfügbaren Modell-IDs gegen die Provider-API (Keys müssen in der Umgebung liegen).

**Warum diese Wahl:** Das Wellenmodell ist deterministisch (Graph + Gates) — der Bedarf ist starke, fokussierte Implementierungs-Agenten (nicht autonome Multi-Agent-Frameworks). Der größte Hebel ist einheitlicher Kontext (`AGENTS.md`), externe Gate-Erzwingung (CI) und ein menschlicher Supervisor. OpenCode + omp decken das mit minimaler Infrastruktur ab.

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
