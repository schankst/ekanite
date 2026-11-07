# Modell-Politik (Provider: OpenRouter & Venice)

**Stand:** 2026-11-07 · Verifiziert gegen die Provider-Kataloge
(OpenRouter: 424 Modelle; Venice: 141 Modelle).

## Grundregeln
1. **Rolle bestimmt Modell** — nicht das Werkzeug. Jede Rolle hat eine Default-Pin; Ausnahmen nur mit Begründung.
2. **Kein Top-Reasoning für Routine.** Mechanische Tasks (Tests, gofmt, Wiring) nutzen Flash-/Mini-Klasse (10–100× günstiger).
3. **Venice bevorzugt, wenn Vertraulichkeit zählt** (Open-Weight-only, kein Tracking); OpenRouter für maximale Modellwahl.
4. **Alias-IDs (`~…-latest`)** sind erlaubt (folgen bestem aktuellen Modell), aber inkonsistent über Zeit — für reproduzierbare Builds explizite IDs pinnen.

## Verifizierte Modell-IDs

| Rolle | OpenRouter | Venice |
|---|---|---|
| Routine (Tests, fmt, Mechanik) | `deepseek/deepseek-v4.1-flash` · `google/gemini-2.5-flash` | `deepseek-v4-flash` · `qwen-3-8-flash` |
| Feature-Worker (F1–F9, MS3/MS4) | `openai/gpt-5.6-luna` · `google/gemini-2.5-pro` · `qwen/qwen3-coder` | `deepseek-v4-pro` · `qwen-3-8-max` |
| Design/Review/Architektur | `openai/gpt-5.6-luna-pro` · `google/gemini-2.5-pro` · `moonshotai/kimi-k2-thinking` | `deepseek-v4-pro` |
| TaskManager-Assist | `google/gemini-2.5-flash` | `qwen-3-8-flash` |

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
