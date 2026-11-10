# Such-Stack: Schema- und Semantik-Vertrag (SQLite + FTS5)

**Status: NORMATIV (Zielzustand).** Beschlossen in Epic #157, festgeschrieben mit #158.
Der Code läuft heute noch auf bleve; dieses Dokument ist der verbindliche Zielzustand, gegen den
#159–#167 implementiert werden. Bei Abweichung gilt dieses Dokument, nicht der Code.

**Artefakt:** Das ausführbare Schema steht in [`docs/sql/search-schema.sql`](sql/search-schema.sql).
Dieses Dokument begründet es und legt die Suchsemantik fest. Beide sind gemeinsam normativ.

## 1. Geltungsbereich und Abgrenzung

- Ein Store **pro (Zeitfenster, Shard)** — die bestehende Architektur (Tages-Indizes, Shard-Routing
  per Hash, Retention per Datei-Drop) bleibt bestehen. Dateilayout/WAL/Wipe regelt #161.
- Dieses Dokument regelt: Spalten/Indizes/PRAGMAs, Zuordnung Parser-Feld → Spalte, Suchsemantik,
  Nicht-Ziele, offene Fragen.
- Nicht hier: Datei-Lifecycle (#161), Datadir-Formatversion/Migration (#162), Query-Dialekt und
  API-Vertrag (#165), Datenminimierung (#163), Messungen (#166).

## 2. Warum SQLite FTS5 (Kurzfassung)

bleve v1 (upsidedown) ist für Syslog strukturell falsch dimensioniert: gemessen 28,6 kB/Event, davon
~72 % aus dem **dynamischen Mapping** der geparsten Felder — bleve speichert, invertiert und
vektorisiert jedes nicht explizit gemappte Feld zusätzlich (`timestamp`, alle FortiOS-Felder).
FTS5 trennt sauber: **Fremdfelder → kompakte Spalten**, **Volltext → ein Index über die Rohzeile**.

| Kennzahl (100k RFC-5424, 4 Shards, Batches 300) | bleve v1 | SQLite FTS5 | Faktor |
|---|---:|---:|---:|
| Disk/Event | 28.588 B | **363 B** | 79× kleiner |
| Ingest | 2.290 Events/s | **51.700 Events/s** | 23× schneller |
| FortiOS (23 Felder, 20k) | 47.427 B/Event | **726 B/Event** | 65× kleiner |

## 3. Schema

Vollständig in [`docs/sql/search-schema.sql`](sql/search-schema.sql). Kern:

- `events`: `id` (Rowedi), `ts`/`recv` als Unix-Nano (`int64` — exakt, sortierbar, 8 Byte),
  `seq` als Tie-Break, die Felder der Parser-Verträge als typisierte Spalten, `msg` (geparster
  Nachrichtentext), `raw` (Rohzeile), `fields` (JSON für den Rest).
- `events_fts`: `fts5(raw, content='events', content_rowid='id', tokenize='unicode61')`.

### 3.1 Warum `raw` genau einmal gespeichert wird

`content='events'` macht den FTS-Index **external content**: FTS5 hält nur den invertierten Index und
liest den Text bei `snippet()`/`highlight()` aus `events.raw`. Eine eigene FTS-Kopie (Default) würde
die Rohzeile ein zweites Mal ablegen — gemessen wäre das ~+250 B/Event ohne Gegenwert.

### 3.2 Warum Felder Spalten sind und nicht FTS-Spalten

Exakte Feldsuche (`app:apache`, `source_ip:10.0.0.3`, `severity:3`) ist ein **Vergleich**, kein
Volltextproblem: `severity:3` darf nicht `3` in `13` oder `3.5` treffen, und eine IP muss als Ganzes
matchen. Spalten liefern das exakt, typisiert und billig; jede zusätzliche FTS-Spalte kostet dagegen
Postings + Dictionary-Einträge pro Feld (der Fehler von bleve).

### 3.3 Parser-Feld → Spalte (verbindlich)

| Parser-Feld | Spalte | Typ | Begründung |
|---|---|---|---|
| `timestamp` | `ts` | INTEGER | wird zur Referenzzeit normalisiert (`input.Event.ReferenceTime`) |
| (Empfang) | `recv` | INTEGER | ReceptionTime |
| (Sequenz) | `seq` | INTEGER | Ordnung bei gleicher `ts` |
| `host`, `app`, `msgid`/`message_id` | `host`, `app`, `msgid` | TEXT | ganze Werte, exakt |
| `source_ip` | `srcip` | TEXT | ganze IP, exakt (nicht tokenisiert) |
| `severity`, `facility`, `priority`, `version` | gleichnamig | INTEGER | Zahlen, exakt/bereichsfähig |
| `pid` | `pid` | INTEGER | Zahl |
| `message` | `msg` | TEXT | Anzeige/Volltext-Ergänzung |
| (Rohzeile) | `raw` | TEXT | Suchbasis + Anzeige |
| **alles andere** (FortiOS-KV u. a.) | `fields` | TEXT (JSON) | siehe 3.4 |

### 3.4 Fremdfelder (`fields`, JSON)

FortiOS liefert 20+ rohe Schlüssel/Wert-Paare. Diese werden **nicht** zu Spalten, sondern als JSON in
`fields` abgelegt (Ergänzung zu den bereits abgebildeten Spalten, ohne Duplikate).

Regeln:
1. **Kein Index** auf `fields` per Default; Suche darauf ist ein Scan
   (`json_extract(fields,'$.logid') = ?`) und damit nicht für heiße Pfade geeignet.
2. Braucht ein Schlüssel indizierte Suche, wird er **zur Spalte befördert** — als bewusste
   Vertragsänderung in diesem Dokument plus Messung (#166). Kein pragmatisches Nachrüsten im Code.
3. Das JSON ist nicht Teil des FTS-Index. Wird Volltext über Fremdfelder gebraucht, ist das eine
   Erweiterung des Vertrags, kein Implementierungsdetail.

`fields` kostet zusätzliche Bytes (JSON-Overhead); der echte Wert wird in #166 gemessen und ersetzt
die Zahl in §6.

## 4. Suchsemantik

### 4.1 Zuordnung Query → SQL

| Query-Form | Ausführung |
|---|---|
| freie Terme (`fail`) | `events_fts MATCH ?` über `raw` |
| Ausdrücke (`AND`/`OR`/`NOT`, `"Phrase"`, `prefix*`, `NEAR()`) | direkt als FTS5-Ausdruck |
| `field:value` (`app:apache`, `severity:3`) | `WHERE spalte = ?` (typisiert) |
| Zeitbereich | `WHERE ts BETWEEN ? AND ?` (Index `events_ts`) |
| Kombination | `JOIN events e ON e.id = f.rowid ... WHERE f MATCH ? AND e.spalte = ? AND e.ts BETWEEN ? AND ?` |

`field:value` und Volltext werden damit **getrennt** ausgewertet und in SQL kombiniert; es gibt keine
Umschreibung von Feldern in FTS-Ausdrücke.

### 4.2 Ordnung und Ranking

- **Ordnungsschlüssel:** `(ts, seq)` — total, ohne Sortierung nach `seq` nicht eindeutig.
- **Relevanz:** `bm25(events_fts)` (kleiner ist besser) als explizite Sortierung, wenn Relevanz
  gewünscht ist.
- **Präsentationsreihenfolge ist ein API-Thema** (#165). Ist-Zustand heute: älteste zuerst
  (`ARCHITECTURE.md` §4.6).

### 4.3 Trefferausgabe

- Dokumentinhalt: `SELECT raw` (bzw. die serialisierten Felder im Minimierungsmodus, #163).
- Kontext: `snippet(events_fts, 0, '…', '…', '…', 12)` / `highlight(...)` — möglich, weil
  external content den Text liefert.

### 4.4 Nebenläufigkeit

- **Ein Schreiber** (der bestehende `Batcher`) — passt zu SQLites Single-Writer-Modell.
- WAL ⇒ Leser blockieren den Schreiber nicht; `busy_timeout` fängt kurze Kollisionen ab.
- Checkpoint/Truncate beim Schließen eines Stores (#161).

## 5. Nicht-Ziele (bewusste Abweichung zu bleve)

1. **Keine Fuzzy-/Edit-Distanz-Suche** und **keine Regex-Suche** — FTS5 kann beides nicht
   (`sqlite.org/fts5.html`); bleves `term~N`/`/re/` entfallen. Ersatz: Prefix-Suche.
2. **Keine Term-Vektoren, keine DocValues, kein `_all`-Sammelfeld.** Volltext gilt der Rohzeile,
   nicht „allen Feldern".
3. **Kein Scoring-Facetten/Highlighting-Modell a la bleve.** Aggregation erfolgt über `GROUP BY`
   (z. B. Top-Hosts) statt Facets.

## 6. Referenzmessung (Spike, Epic #157)

**Umgebung:** linux/amd64, 2 vCPU (Intel Xeon Skylake), Go 1.26, Treiber `modernc.org/sqlite`
v1.62.0 (cgo-frei, `SQLITE_ENABLE_FTS5=1`), WAL, `synchronous=NORMAL`, `cache_size=-65536`.

**Last:** 100.000 RFC-5424-Events (Beispielzeile mit Structured Data), echte Parser-Felder,
4 Shards, Batches 300, `optimize()` am Ende.

| Größe | Wert |
|---|---:|
| Gesamtgröße Store | 36.343.808 B |
| **Disk/Event** | **363 B** |
| davon Spalten + Rohzeile | 236 B |
| davon FTS5-Index | ~127 B |
| Ingest | 1,82–1,93 s → **~51.700 Events/s** |
| FortiOS (23 Felder, 20k) | 726 B/Event (vs. 47.427 B bei bleve) |
| Linearität | 20k: 393 B/Event · 100k: 363 B/Event (fallend, nicht superlinear) |

Abfragen am 100k-Index (Trefferlimit 5000):

| Abfrage | Treffer | Zeit |
|---|---:|---:|
| Volltext | 5000 | 2,1 ms |
| Phrase | 5000 | 2,3 ms |
| Prefix | 5000 | 5,6 ms |
| FTS + Zeitfenster | 5000 | 2,6 ms |
| Feld exakt | 5000 | 1,4 ms |

**Belastbarkeit:** Der Spike-Code war Wegwerf (nicht committet). Die Werte sind mit
`docs/sql/search-schema.sql` und den Parametern oben reproduzierbar. Die Last enthält je Event einen
zusätzlichen eindeutigen Term (`#N`) und ist damit eine **konservative Obergrenze** für reale
Syslog-Last. #166 ersetzt diese Zahlen durch CI-Messungen.

## 7. Offene Fragen (bewusst delegiert)

| Frage | Wird geklärt in |
|---|---|
| Extern sichtbares Doc-ID-Format (heute 32-Hex-`DocID`; mit Rowedi ändert es sich) | #165 |
| Präsentationsreihenfolge (heute älteste zuerst) und Relevanz-Option | #165 |
| Paging: `LIMIT/OFFSET` vs. Keyset (`ts < ?`); harte Obergrenze (heute 10.000/Request) | #165, #30 |
| Query-Timeout/-Abbruch: `ctx`-Support und `sqlite3_interrupt` des Treibers verifizieren | #160 |
| `page_size` (4096 vs. 8192/16384) und weitere Indizes — nur mit Messung | #166 |
| `fields`-JSON: Byte-Kosten und Bedarf für beförderte Schlüssel | #166 |
| `-rawmode drop`: `raw IS NULL` ⇒ kein Volltext; eigener anonymisierter Index nötig? | #163 |
| `optimize()`-Zeitpunkt (je Batch, beim Rotieren des Zeitfensters, periodisch) | #161 |

## 8. Referenzen

- Epic #157 · Schema-Artefakt `docs/sql/search-schema.sql` · #159 (Insert-Pfad) · #160 (Suche)
- #161 (Lifecycle/Retention/Wipe) · #162 (Datadir-Vertrag) · #163 (Minimierung) · #165 (Dialekt/API)
- #166 (Messungen) · #167 (ADR/ARCHITECTURE)
- Vorlage: `docs/ARCHITECTURE.md` §5
