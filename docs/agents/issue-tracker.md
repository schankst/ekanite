# Issue tracker: GitHub

Issues und Specs dieses Repos leben als GitHub Issues in `schankst/ekanite`. Alle Operationen über die `gh`-CLI.

## Konventionen

- **Issue anlegen**: `gh issue create --title "..." --body "..."` (Heredoc für mehrzeilige Bodies).
- **Issue lesen**: `gh issue view <nummer> --json number,title,body,labels,comments`.
- **Issues listen**: `gh issue list --state open --json number,title,labels --jq 'sort_by(.number)[] | ...'` mit `--label`/`--state`-Filtern.
- **Sub-Issue**: `Part of #<parent>` oben in der Child-Body (Sub-Issues nativ in diesem Setup nicht genutzt).
- **Blocking (native GitHub-Dependencies)**: `gh issue edit <child> --add-blocked-by <blocker>` (gh 2.94+); älteres gh via `gh api --method POST repos/schankst/ekanite/issues/<child>/dependencies/blocked_by -F issue_id=<blocker-db-id>` — `<blocker-db-id>` = numerische DB-ID (`gh api .../issues/<n> --jq .id`), nicht `#number`. Fallback: `Blocked by: #<n>`-Zeile oben in der Body. Ein Issue ist unblocked, wenn alle Blocker geschlossen sind.
- **Offene Blocker zählen**: `gh api repos/schankst/ekanite/issues/<n> --jq .issue_dependencies_summary.blocked_by` (0 = startbar).
- **Kommentieren**: `gh issue comment <nummer> --body "..."`.
- **Labels**: `gh issue edit <nummer> --add-label "..."` / `--remove-label "..."`.
- **Schließen**: erst Kommentar mit Begründung, dann `gh issue close <nummer>`.

Repo aus `git remote -v` ableiten; `gh` erledigt das automatisch im Clone. Ziel immer `schankst/ekanite` — nie in Upstream `ekanite/ekanite` (archiviert).

## Pull requests als Triage-Oberfläche

**PRs als Request-Surface: nein.** (`/triage` liest dieses Flag.) Externe PRs werden nicht als Feature-Requests behandelt.

## Wenn ein Skill "publish to the issue tracker" sagt
GitHub-Issue anlegen (wie oben).

## Wenn ein Skill "fetch the relevant ticket" sagt
Wie unter **Issue lesen**.

## Wayfinding-Operationen (`/wayfinder`)
- **Map**: ein Issue mit Label `wayfinder:map`, Body = Notes/Decisions/Fog.
- **Child-Ticket**: Issue mit `Part of #<map>` + `wayfinder:<type>`-Label (research/prototype/grilling/task).
- **Frontier-Query**: offene Children ohne offene Blocker und ohne Assignee (siehe Blocking), erste in Map-Reihenfolge.
- **Claim**: `gh issue edit <n> --add-assignee @me`.
- **Resolve**: Kommentar mit Antwort → `gh issue close <n>` → Kontext-Pointer in Map-Decisions.
