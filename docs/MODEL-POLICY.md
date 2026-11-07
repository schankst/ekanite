# Modell-Politik (Provider: OpenRouter & Venice)

**Stand:** 2026-11-07 · Verifiziert gegen die Provider-Kataloge
(OpenRouter: 424 Modelle; Venice: 141 Modelle).

## Grundregeln
1. **Rolle bestimmt Modell** — nicht das Werkzeug. Jede Rolle hat eine Default-Pin; Ausnahmen nur mit Begründung.
2. **Kein Top-Reasoning für Routine.** Mechanische Tasks (Tests, gofmt, Wiring) nutzen Flash-/Mini-Klasse (10–100× günstiger).
3. **Venice bevorzugt, wenn Vertraulichkeit zählt** (Open-Weight-only, kein Tracking); OpenRouter für maximale Modellwahl.
4. **Alias-IDs (`~…-latest`)** sind erlaubt (folgen bestem aktuellen Modell), aber inkonsistent über Zeit — für reproduzierbare Builds explizite IDs pinnen.

## Verifizierte Modell-IDs

| Rolle | OpenRouter-ID | $/1M in · out | Venice-ID | $/1M in · out (Cache) |
|---|---|---|---|---|
| Routine (Tests, fmt, Mechanik) | `deepseek/deepseek-v4.1-flash` | 0,05 · 0,57 | `deepseek-v4-flash` | 0,14 · 0,28 |
| Routine (Alt.) | `google/gemini-2.5-flash` | 0,30 · 2,50 | `qwen-3-8-flash` | 0,14 · 0,49 |
| Feature-Worker (F1–F9, MS3/MS4) | `openai/gpt-5.6-luna` | 0,20 · 1,20 | `deepseek-v4-pro` | 1,65 · 3,30 |
| Feature (Alt.) | `google/gemini-2.5-pro` | 1,25 · 10,00 | `qwen-3-8-max` | 2,50 · 7,50 |
| Feature (Alt.) | `qwen/qwen3-coder` | 0,30 · 1,00 | — | — |
| Design/Review/Architektur | `openai/gpt-5.6-luna-pro` | 0,20 · 1,20 | `deepseek-v4-pro` | 1,65 · 3,30 |
| Review (Alt.) | `moonshotai/kimi-k2-thinking` | 0,60 · 2,50 | — | — |
| TaskManager-Assist | `google/gemini-2.5-flash` | 0,30 · 2,50 | `qwen-3-8-flash` | 0,14 · 0,49 |

Kostenquellen: OpenRouter `/api/v1/models` (pricing, pro Token → ×1M) und Venice `/models` (`model_spec.pricing.usd`, pro 1M); Venice-Cache-Input z. B. `deepseek-v4-flash` 0,028 $/1M. Preise können sich ändern — Stichtag 2026-11-07.

## Pins je Werkzeug (Soll)
| Werkzeug | Default | Hinweis |
|---|---|---|
| **opencode** | `deepseek/deepseek-v4.1-flash` (Provider openrouter) | über `~/.config/opencode/opencode.jsonc` |
| **pi** | `~deepseek/deepseek-v4-flash-latest` (bereits gesetzt) | Settings: `~/.pi/agent/settings.json` |
| **Cline** | Venice `deepseek-v4-pro` (falls IDE-Arbeit) | Modellwahl in UI |

## Kostenkontrollen
- Pro Provider Budget via Wöchentlichem Review (OpenRouter Dashboard / Venice-Usage).
- `thinkingLevel: off` bei Routine (pi bereits so).
- Batch-/Cache-Optionen nutzen, wo angeboten (`:batch`-Suffixe auf OpenRouter).
