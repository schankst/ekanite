-- Ekanite — Such-/Speicher-Schema (Zielzustand: SQLite + FTS5)
--
-- NORMATIV. Der Vertrag (Semantik, Begruendungen, Messwerte) steht in
-- docs/SEARCH-STACK.md (Issue #158). Dieses File ist ausfuehrbar und die
-- Vorlage fuer die Implementierung (#159): Schema und PRAGMAs muessen
-- damit uebereinstimmen.
--
-- Aufbau: Erklaerungen stehen ausschliesslich in eigenen Kommentarzeilen,
-- damit das File von jedem Splitter (Kommentarzeilen verwerfen, an ';'
-- trennen) verarbeitet werden kann. Keine nachgestellten Kommentare.
--
-- Geltungsbereich: EIN Store je (Zeitfenster, Shard). Retention faellt ganze
-- Dateien, deshalb kein VACUUM und keine zeilenweise Loeschlogik (#161).
--
-- Referenzmessung dieser Konfiguration (Epic #157): 100.000 RFC-5424-Events,
-- 4 Shards, Batches 300 -> 363 B/Event, ~51.700 Events/s Ingest.

-- Verbindungseinstellungen, pro Verbindung und vor dem ersten Schreiben:
-- WAL  = Leser parallel zum einen Schreiber
-- NORMAL = letzter Batch kann bei Stromausfall verloren gehen
-- busy_timeout = 5000 ms, verhindert sofortiges SQLITE_BUSY
-- cache_size = -65536 (negative Zahl = KiB), 64 MiB Seitencache
-- page_size muss VOR dem ersten CREATE TABLE gesetzt werden, Default = 4096
PRAGMA journal_mode = WAL;
PRAGMA synchronous = NORMAL;
PRAGMA busy_timeout = 5000;
PRAGMA cache_size = -65536;

-- events: genau eine Zeile je Ereignis
-- id   Rowedi; das extern sichtbare Doc-ID-Format ist #165
-- ts   Referenzzeit als Unix-Nano (int64: exakt, sortierbar, 8 Byte)
-- recv Empfangszeit als Unix-Nano
-- seq  Eingangssequenz, Tie-Break bei gleicher ts
-- msg  geparster Nachrichtentext (Parser-Feld "message")
-- raw  vollstaendige Rohzeile, NULL bei "-rawmode drop" (#163)
-- fields JSON mit allen uebrigen Parser-Feldern (z. B. FortiOS-KV)
CREATE TABLE events (
    id       INTEGER PRIMARY KEY,
    ts       INTEGER NOT NULL,
    recv     INTEGER NOT NULL,
    seq      INTEGER NOT NULL,
    host     TEXT,
    app      TEXT,
    pid      INTEGER,
    severity INTEGER,
    facility INTEGER,
    priority INTEGER,
    version  INTEGER,
    msgid    TEXT,
    srcip    TEXT,
    msg      TEXT,
    raw      TEXT,
    fields   TEXT
);

-- Feldsuche laeuft exakt ueber Spalten. Standardmaessig NUR diese beiden
-- Indizes; jeder weitere Index erhoeht Disk/Event und braucht eine Messung (#166).
CREATE INDEX events_ts ON events (ts);
CREATE INDEX events_severity_ts ON events (severity, ts);

-- events_fts: Volltext ueber die Rohzeile.
-- external content = die Rohzeile wird genau EINMAL gespeichert (in events.raw);
-- FTS5 haelt nur den invertierten Index und liest den Text fuer snippet()/highlight().
-- tokenize unicode61 = casse-/akzentunabhaengig, Trenner sind Nicht-Alphanumerika.
-- Bewusst NICHT als FTS-Spalten: IPs, Severity, Facility usw. — exakte Feldsuche
-- laeuft ueber Spalten; das spart, was bleve pro Feld ~1,5-2 kB gekostet hat.
CREATE VIRTUAL TABLE events_fts USING fts5 (
    raw,
    content = 'events',
    content_rowid = 'id',
    tokenize = 'unicode61'
);

-- Betriebshinweise (kein DDL):
-- 1. Nach einem Bulk-Import einmal ausfuehren (verdichtet den Index-Rest):
--    INSERT INTO events_fts(events_fts) VALUES ('optimize');
-- 2. Insert je Dokument, innerhalb der Batch-Transaktion:
--    INSERT INTO events (ts, recv, seq, host, app, pid, severity, facility,
--                        priority, version, msgid, srcip, msg, raw, fields)
--    VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?);
--    INSERT INTO events_fts (rowid, raw) VALUES (last_insert_rowid(), ?);
--    Bei raw IS NULL entfaellt der FTS-Insert; die Zeile bleibt ueber Felder findbar.
