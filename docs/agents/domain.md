# Domain Docs

Wie die Engineering-Skills die Domain-Doku dieses Repos konsumieren.

## Vor dem Erkunden lesen

- **`GLOSSARY.md`** am Root, oder
- **`GLOSSARY-MAP.md`** am Root, falls vorhanden (multi-context),
- **`docs/adr/`**: ADRs im Arbeitsbereich.

Existieren diese Dateien nicht, **still weitermachen** — nicht fehlen melden, nicht frühzeitig anlegen. `/domain-modeling` erzeugt sie lazy, wenn Begriffe/Entscheidungen wirklich aufgelöst werden.

## Datei-Struktur (single-context)

```
/
├── GLOSSARY.md
├── docs/adr/
│   ├── 0001-*.md
│   └── 0002-*.md
└── src/
```

Dieses Repo ist **single-context** (kein Monorepo, kein `pnpm-workspace.yaml`/`package.json`-Workspace).

## Glossar-Vokabular nutzen

Bei Domain-Begriffen (Issue-Titel, Refactor, Testname) die Terminologie aus `GLOSSARY.md` verwenden, keine Synonyme.

Fehlt ein Begriff im Glossar: Signal — entweder neue Sprache erfunden (überdenken) oder echte Lücke (für `/domain-modeling` notieren).

## ADR-Konflikte flaggen

Widerspricht ein Output einem bestehenden ADR, explizit oberflächlich behandeln statt still zu überstimmen:

> _Contradicts ADR-0007 (…), aber Reopen lohnt, weil…_
