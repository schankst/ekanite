# Modell-Politik (verbindliche Auswahl)

**Stand:** 2026-11-07 · Gültig für den Fork `schankst/ekanite`.
Die folgenden Modelle sind festgelegt und gegen die Provider verifiziert (IDs existieren, Preise aktuell abgefragt).

## Festgelegte Modell-Auswahl

| Rolle | Modell | Provider | $/1M in · out | Kontext |
|---|---|---|---|---|
| Routine (Tests, fmt, Mechanik) | `deepseek/deepseek-v4.1-flash` | OpenRouter | 0,05 · 0,57 | 1.048.576 |
| Feature (F1–F9, MS3/MS4) | `openai/gpt-5.6-luna` | OpenRouter | 0,20 · 1,20 | 1.050.000 |
| Review/Architektur | `openai/gpt-5.6-luna-pro` | OpenRouter | 0,20 · 1,20 | 1.050.000 |
| TaskManager | `qwen-3-8-flash` | Venice | 0,14 · 0,49 | 1.000.000 |

## Regeln

1. **Rolle bestimmt Modell** — jede Rolle nutzt ausschließlich ihr festgelegtes Modell; Abweichung nur mit Begründung.
2. **Kein Top-Reasoning für Routine** — `deepseek-v4.1-flash` für mechanische Tasks hält die Kosten minimal.
3. **Venice für TaskManager** (Vertraulichkeit) — die übrigen drei Rollen laufen über OpenRouter.
4. **Kostenkontrolle:** wöchentliche Review über die Provider-Dashboards; bei Erreichen eines Monatsbudgets (Festlegung im Projekt) wird die Feature-/Review-Stufe geprüft.

## Pins je Werkzeug

| Werkzeug | Default | Provider |
|---|---|---|
| **opencode** | `deepseek/deepseek-v4.1-flash` | openrouter |
| **pi** | `~deepseek/deepseek-v4-flash-latest` | openrouter |
| **Cline** (falls IDE) | `qwen-3-8-flash` | venice |

## Verprobung (2026-11-07)

Alle vier IDs wurden live gegen die jeweilige Provider-API abgefragt und existieren mit den angegebenen Preisen und Kontextlängen. Preise können sich ändern — bei Abweichung im Katalog prüfen.