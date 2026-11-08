# AGENTS.md — Ekanite

Agentische Umsetzung des Wellenmodells. Die ausführlichen Rollen-, Eigentums- und Ablaufbeschreibungen stehen in `docs/AGENTS.md` (Worker/Supervisor/TaskManager) und `docs/ROADMAP.md` (Wellen, Gates).

## Agent skills

### Issue tracker

Arbeit wird als GitHub Issues getrackt; alle Operationen über die `gh`-CLI. Siehe `docs/agents/issue-tracker.md`.

### Triage labels

Fünf kanonische Triage-Rollen, Label = Rolle: `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`. Siehe `docs/agents/triage-labels.md`.

### Domain docs

Single-context: ein `GLOSSARY.md` am Root + `docs/adr/`. Aktuell noch keine vorhanden — werden von `/domain-modeling` lazy angelegt. Siehe `docs/agents/domain.md`.

### Lagebild

Statusübersicht des Projekts lebt in `docs/LAGEBILD.md` (lebend, wird ersetzt, nicht angereichert). Bei „Lagebild“/„Statusübersicht“-Anfragen den Skill `lagebild` nutzen — Schema und Erzeugung siehe `~/.agents/skills/lagebild/SKILL.md`.