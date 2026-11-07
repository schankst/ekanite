# Glossary

*Dieser Abschnitt wird lazy durch `/domain-modeling` befüllt, sobald Begriffe aufgelöst werden. Derzeit keine fixen Domain-Begriffe definiert.*

Vorgezogene Begriffe aus dem aktuellen Kontext (informell):

- **DocID**: 32-hex Zeitordnung (`msw+lsw` aus ReferenceTime/Sequence), bestimmt Shard-Routing + Sortierreihenfolge.
- **Index**: Tages-Zeitscheibe (`20060102_1504`), mit `endtime`-Metadatei; mindeste Retention-Einheit.
- **Shard**: einzelnes bleve-Index-Verzeichnis unter einem Index; FNV-32a-Routing.
- **Batcher**: sammelt Events zu Batches (`-batchsize`/`-batchtime`), schreibt an die Engine.
- **ReferenceTime**: geparste Event-Zeit (RFC3339) oder Empfangszeit; Grundlage für Index-Routing.
