# Ekanite — Architektur (normativ)

> **Status:** Normatives Referenzdokument für Maintainer (Issue #79, MS6/C1).  
> **Geltungsbereich:** Ableitungen aus dem aktuellen Code auf Branch `task/79`. Wo der Code uneindeutig oder widersprüchlich ist, ist dies explizit markiert (`[INFERENCE]`, `[RISIKO]`). Dieses Dokument erhebt keinen Absolutheitsanspruch; bei Konflikt gilt der Code, Abweichungen sind als Issue zu erfassen.

## 1. Überblick

Ekanite ist ein Syslog-Index-/Suchdienst in Go ohne externe Laufzeitabhängigkeiten (eingebettete Indizes via [bleve](https://github.com/blevesearch/bleve)). Er nimmt Logzeilen entgegen, indexiert sie zeitlich partitioniert und stellt sie über eine Suchschnittstelle bereit.

| Komponente | Rolle |
|---|---|
| **Eingang (Collector)** | Nimmt Zeilen über TCP/UDP (optional TLS für TCP) entgegen (`input.Collector`, `input.NewCollector`). |
| **Parser/Delimiter** | Trennt Nachrichten (Syslog-Delimiter) und parst Formate `syslog`/RFC5424 oder `fortios` (`input.LogHandler`/`NewParser`, `parser.RFC5424`, `parser.FortiOS`). |
| **Ingest/Batching** | Puffert Eingangsereignisse und bündelt sie zu Index-Batches (`ekanite.Batcher`, `NewBatcher`, `Start`, `C`). |
| **Index (Engine/Index/Shard)** | Schreibt Batches zeitbereichsbezogen in bleve-Shards (`Engine`, `Index`, `Shard`, `Event`). |
| **Retention** | Entfernt abgelaufene Zeit-Indizes periodisch (`Engine.runRetentionEnforcement`/`enforceRetention`, `Index.Expired`, `DeleteIndex`). |
| **Query** | Führt Suchen über die Zeit-Indizes aus und streamt Treffer (`Engine.Search`, TCP-`Server`, HTTP-`HTTPServer`). |

## 2. Datenfluss (Pipeline)

```mermaid
flowchart LR
  C["Collector TCP/UDP/TLS<br/>(input.NewCollector → Start)"] --> P["Parser/Delimiter<br/>(SyslogDelimiter, LogHandler.Parse)"]
  P -->|input.Event + ReceptionTime/Sequence/SourceIP| BC["Batcher.C() chan *input.Event<br/>(gepuffert: maxpending)"]
  BC --> B["Batcher (size/timeout)<br/>send → indexer.Index"]
  B --> E["Engine.Index([]*Event)<br/>Demux nach Zeit-Index"]
  E --> I["Index (Zeitbereich)<br/>→ Shard-Routing fnv32a(DocID)"]
  I --> S["Shard.Index → bleve Batch<br/>(Index + SetInternal Source)"]
  R["Retention-Sweep (stündlich)<br/>EngineRetention/enforceRetention"] -.->|löscht abgelaufene| I
  Q["Query TCP/HTTP"] --> ES["Engine.Search(query)"]
  ES -->|bleve QueryStringQuery pro Index| I
```

**Startup-Reihenfolge** (`cmd/ekanited/main.go`, `main`): Datenverzeichnis/Retention parsen → `GOMAXPROCS` → Diag-Server (`startDiagServer`, expvar/pprof) falls `-diag` → `NewEngine` + `Open()` (lädt bestehende Indizes, startet Retention-Goroutine) → Query-Server TCP/HTTP falls konfiguriert → `NewBatcher` + `Start` + Fehler-Drain → TCP-Collector → UDP-Collector → Profiling → auf Signal warten → `engine.Close()` → Profiling stoppen. Collector-Start erfolgt **nach** dem Batcher; Query-Server starten nach `engine.Open()`.

## 3. Komponenten & Verantwortlichkeiten

- **`Engine`** (`engine.go`): Hält `indexes` (zeitliche `*Index`-Liste, sortiert), Konfiguration (`NumShards`, `IndexDuration` 24h, `RetentionPeriod`, `RetentionCheckInterval` 1h), Lifecycle (`Open`/`Close`). `Index()` demultiplext Batches auf passende Zeit-Indizes, erzeugt fehlende Indizes lazy, indexiert Sub-Batches parallel. `Search()` iteriert Indizes und streamt Dokumentquellen. `Total()` aggregiert Shard-DocCounts.
- **`Index`** (`index.go`): Repräsentiert einen Zeitbereich `[startTime, endTime)` (Verzeichnisname `20060102_1504`, Endzeit in Datei `endtime`). Enthält `Shards []*Shard` und ein `bleve.IndexAlias` über alle Shards für Suche. Routet Dokumente per Hash auf Shards (`Shard(docId)`) und führt Suche/Fetch aus.
- **`Shard`** (`index.go`): Dünne Hülle um einen `bleve.Index` (Pfad pro Shard, z. B. `0000`). `Open()` erzeugt/öffnet Mapping (`buildIndexMapping`, Analyzer `ekanite`). `Index()` schreibt per `bleve.Batch` (`Index` + `SetInternal` für Source). Kein eigener Mutex im Code.
- **`Batcher`** (`engine.go`): Nimmt `*input.Event` auf Kanal `C()` (Kapazität `max`) entgegen. Flush bei `size` Elementen oder nach `duration` (Timer, Reset bei erstem Element des Batches). Ruft `EventIndexer.Index` (implementiert von `Engine`).
- **`Collector`** (`input/collector.go`): `TCPCollector`/`UDPCollector` erzeugen Parser (`NewParser(format)`, Formate `syslog`/`fortios`), setzen `ReceptionTime = time.Now().UTC()`, `Sequence = atomic.AddInt64(&sequenceNumber,1)` (global, initial `time.Now().UnixNano()`), `SourceIP`, und senden akzeptierte Events in den Batcher-Kanal. TCP nutzt `SyslogDelimiter` und Read-Deadline (`newlineTimeout` 1000ms).
- **Query-Server** (`server.go`, `server_http.go`): TCP liest zeilenweise Queries, schreibt Trefferzeilen und beendet mit `\n\n`. HTTP (`POST` Feld `query`) sammelt alle Treffer und rendert HTML; `GET`/`HEAD` liefert Index. Beide nutzen das `Searcher`-Interface (`Engine`).

## 4. Invarianten

### 4.1 Zeitnormalisierung
- **Referenzzeit:** `input.Event.ReferenceTime()` (`input/event.go`): memoisiert in `referenceTime`. Ist `Parsed == nil`, gilt `ReceptionTime`. Ist `Parsed != nil`, wird `time.Parse(time.RFC3339, e.Parsed["timestamp"].(string))` versucht; bei Parse-Fehler Fallback auf `ReceptionTime`, sonst der geparste Zeitstempel.
- **Ohne/fehlerhafter Zeitstempel:** `Parsed == nil` → `ReceptionTime`. `[RISIKO, Codepfad]` Der Zugriff `e.Parsed["timestamp"].(string)` erfolgt ohne Guard: fehlt der Schlüssel oder ist der Wert kein `string`, löst die Go-Typzusicherung eine Panic aus (kein Fallback). RFC5424 legt `timestamp` als Rohwert `m[3]` ab (`parser/rfc5424.go`); ist dieser nicht RFC3339, scheitert das RFC3339-Parsing und es greift der Fallback auf `ReceptionTime`. FortiOS normalisiert `date`/`time` zu RFC3339 (`parser/fortios.go`, `fortiOSTimestamp`/`Parse`).
- **`ReceptionTime`:** Wird beim Empfang auf `time.Now().UTC()` gesetzt (Collector).

### 4.2 DocID- und Shard-Aufteilung
- **DocID:** `Event.ID()` (`event.go`) = `fmt.Sprintf("%016x%016x", uint64(e.ReferenceTime().UnixNano()), uint64(e.Sequence))` (`DocID`, 32 Hex-Zeichen). Most-significant 16 Zeichen = Referenzzeit in Nanosekunden, least-significant 16 = Sequenz. `DocIDs.Less` sortiert nach msw, dann lsw (zeit-/sequenzstabil).
- **Zeit-Index (grobe Partition):** `Engine` wählt/erzeugt pro `ev.ReferenceTime()` einen `Index` (`indexForReferenceTime`/`createIndexForReferenceTime`). Bereich: `start = rt.Truncate(IndexDuration).UTC()`, `end = start + IndexDuration` (`DefaultIndexDuration` 24h, `engine.go`). Enthält-Zeitraum: `[startTime, endTime)` (`Index.Contains`). Namenskollision gleicher Startzeit wird durch Verschieben von `startTime` auf `idx.endTime` behandelt (`createIndex`) — `[INFERENCE]` der Kommentar im Code weist auf einen möglichen Restfall bei weiterer Kollision hin (`XXX`); kein Schleifen-Ausgleich im Code.
- **Bleve-Subshards (Routing):** Innerhalb eines `Index` wird ein Dokument einem `Shard` zugeordnet: `fnv.New32a()` über `[]byte(docId)`, `Sum32() % uint32(len(i.Shards))` (`Index.Shard`). Anzahl Shards pro Zeit-Index = `Engine.NumShards` (Default in `engine.NewEngine`: 16; **effektiver CLI-Default über `cmd/ekanited/main.go`: `-numshards` = 4**, `DefaultNumShards=4`).
- **Suche über Partitionen:** `Engine.Search` iteriert `e.indexes` **rückwärts** (`len-1` → `0`). `Indexes` sind sortiert nach absteigendem `endTime` (bei Gleichstand absteigendem `startTime`, `Indexes.Less`), d. h. Index 0 ist der jüngste Bereich; die Rückwärtsiteration durchsucht damit vom **ältesten zum jüngsten** Zeitbereich (Kommentar: „starting with the earliest in time“). Pro Index liefert `Index.Search` per bleve Treffer-IDs, sortiert sie als `DocIDs` aufsteigend und streamt die Quellen.

### 4.3 Lock-Reihenfolge & -Stile
- **`Engine.mu sync.RWMutex`** schützt `e.indexes` und Open-Zustand-nahe Operationen: `Total`/`Search` halten `RLock`; `enforceRetention` hält `Lock`; `Open`/`Close` mutieren Indexliste (Lifecycle-Kontext).
- **`Engine.Index` (Lock-Upgrade-Muster, #13-relevant):** Hält eingangs `RLock` (defer `RUnlock`). Fehlt ein Ziel-Index, wird innerhalb einer anonymen Funktion `RUnlock()` → `Lock()` genommen, unter `Lock` re-geprüft und ggf. `createIndexForReferenceTime` ausgeführt, dann `Unlock()` → `RLock()` wiederhergestellt. `[RISIKO, #13]` Dieses manuelle Downgrade/Upgrade ist fehleranfällig (Fenster zwischen Unlock/Lock, Halten von RLock über I/O-nahe Arbeit) und ist als bekannte Race-Stelle zu behandeln; Reihenfolgeänderungen nur mit Bezug auf #13.
- **`Index`/`Shard`:** Kein eigener Mutex im Code. `Index.Index` parallelisiert pro Shard (`sync.WaitGroup`); `Shard.Index` nutzt `bleve.Batch` auf dem Shard-Index. `[INFERENCE]` Nebenläufigkeitssicherheit auf Shard-Ebene hängt von bleve bzw. der aufrufenden Serialisierung ab; der Code serialisiert Schreibzugriffe nicht explizit über einen Shard-Lock.

### 4.4 Batch-Semantik
- **Parameter (CLI-Defaults, `main.go`):** `-batchsize` 300, `-batchtime` 1000 ms, `-maxpending` 1000 (Kanalpuffer `NewBatcher`: `make(chan *input.Event, max)`).
- **Flush:** `Batcher.Start` flusht, wenn `len(batch) == size` (Timer gestoppt) oder bei Timer-Ablauf (`batchTimeout`). Erfolgreicher Flush: `EventIndexer.Index(batch)` liefert `nil` → Stats inkrementiert, **neues leeres Batch** angelegt. `errChan` erhält den `err`-Wert nur im Erfolgszweig des aktuellen Codes (`if errChan != nil { errChan <- err }` nach erfolgreichem Index); bei Indexfehler wird `batchIndexedError` gezählt und `send()` kehrt zurück **ohne** Batch-Reset.
- **Garantien/Durabilität:** Ein Event gilt erst als verarbeitet, wenn `Engine.Index` die Shard-Batches (`Shard.Index` → `s.b.Batch`) erfolgreich abschließt. `[RISIKO]` Bei Fehlern aus `Engine.Index`/`Shard.Index` bleibt das Batch im Fehlerpfad des aktuellen `send()`-Codes erhalten; je nach Triggerpfad kann dies zu Wiederholungsversuchen/Duplikaten oder (bei size-Trigger: Timer gestoppt, Batch nicht zurückgesetzt) zu ausbleibenden weiteren size-Flushes führen. Kein End-to-End-Ack an den Collector über den Kanal hinaus.

### 4.5 Retention
- **Parameter:** `-retention` Default `168h` (`main.go`); Hilfe/Flagtext nennt „Minimum is 24 hours“, **aber im Code wird kein Minimum erzwungen** (nur `time.ParseDuration` in `main`; `engine.DefaultRetentionPeriod` = 24h). `[INFERENCE]` Werte < 24h sind technisch parsebar, sofern gültige Duration — beabsichtigtes Minimum ist im Code nicht durchgesetzt.
- **Ablauf:** `Engine.runRetentionEnforcement` prüft alle `RetentionCheckInterval` (1h, `engine.go`). `enforceRetention` nimmt `Engine.mu.Lock`, prüft `i.Expired(time.Now().UTC(), e.RetentionPeriod)` (`endTime.Add(r).Before(t)`), ruft `DeleteIndex` (schließt, dann `os.RemoveAll(i.path)`) und entfernt den Index aus `e.indexes`.
- **Konsistenz:** Löschung erfolgt unter Engine-Lock; parallele Suche/Indexierung koordinieren über `Engine.mu`. `[INFERENCE]` Laufende Such-Goroutinen halten ggf. Index-Referenzen über den Snapshot hinaus; DeleteIndex schließt den Index — exakte Nebenläufigkeitswirkung beim gleichzeitigen Zugriff ist aus dem Code allein nicht vollständig spezifiziert.

### 4.6 Reihenfolge-Garantie (Suche)
- Ergebnisse werden nach **Event-Zeit (Referenzzeit)** geliefert, nicht nach Ingest-Reihenfolge: `Engine.Search` traversiert Zeit-Indizes vom ältesten zum jüngsten; `Index.Search` sortiert Treffer-`DocID`s aufsteigend (`DocIDs.Less`, msw = `ReferenceTime().UnixNano()`, lsw = `Sequence`) und streamt anschließend die Dokumentquellen. `[INFERENCE]` Globale Ordnung über Indexgrenzen folgt der Zeitbereichsaufteilung; innerhalb eines Bereichs der DocID-Ordnung.

## 5. Such-API

- **TCP** (`server.go`, `NewServer`/`Start`): Pro Verbindung werden Zeilen gelesen (CR/LF getrimmt), leere Zeilen ignoriert. Jede Query wird an `Searcher.Search` delegiert; Treffer (`string` pro Dokumentquelle) werden zeilenweise geschrieben, Fehler als Text in die Verbindung. Abschluss einer Query-Antwort: zwei Zeilenumbrüche (`\n\n`). Keine Authentifizierung im Code.
- **HTTP** (`server_http.go`, `NewHTTPServer`/`Start`): `GET`/`HEAD` liefert UI; `POST` mit Formularfeld `query` führt Suche aus, sammelt **alle** Treffer in einen Slice und rendert HTML (inkl. Trefferanzahl/Dauer). Leere Query fällt auf Index zurück. Fehler → HTTP 500/400. Keine Authentifizierung im Code.
- **Query-Sprache (effektiv):** `Index.Search` baut `bleve.NewQueryStringQuery(q)` und sucht über den Shard-Alias (`searchRequest.Size = maxSearchHitSize` 10000). Die UI verweist auf die bleve-Query-String-Syntax. Das Paket `query/` implementiert einen eigenen Parser/AST (`query.NewParser`/`Parse`, Field/Binary/Paren-Ausdrücke) — **dieser wird im aktuellen Suchpfad nicht verwendet** (separater Codepfad/tote Umschaltlogik, siehe #32).
- **Fehler-/Leerfälle:** Bleve-Suchfehler brechen den Engine-Scan ab (Log + `break`). Dokument-Fetch-Fehler pro ID werden geloggt und brechen den aktuellen Index-Lauf ab. `[INFERENCE]` Max. 10000 Treffer pro Index-Suchrequest (`maxSearchHitSize`); größere Treffermengen werden durch bleve/Request-Limit begrenzt.

## 6. Bekannte Grenzen & Risiken

- **#32 – tote/abweichende Query-Logik:** `query/`-Paket (eigener Parser) ist nicht in den aktiven Suchpfad (`Engine`/`Index` → `bleve.NewQueryStringQuery`) integriert; Suchsemantik folgt bleve, nicht dem `query/`-AST.
- **#13 – Lock-Race/Lock-Muster:** Manuelles `RUnlock`/`Lock`/`Unlock`/`RLock` in `Engine.Index` beim Nach-Anlegen von Zeit-Indizes (Race-Fenster/Komplexität).
- **#14 / #52 – keine Authentifizierung:** TCP- und HTTP-Query-Endpunkte sowie (je nach Deployment) Diag-Bindung sind im Code nicht authentifiziert.
- **Exponiertes pprof/expvar:** Bei aktiviertem `-diag` bedient `status.Service` `/debug/vars` und `/debug/pprof/*` (`status/service.go`); `main` importiert zudem `_ "net/http/pprof"`.
- **Datenschutz:** Siehe `docs/PRIVACY.md` (Logdaten, Betriebs-/Zugriffsaspekte); Query/Diag-Exposition beachten.
- **Batch-Fehlerpfad:** `Batcher.send` setzt das Batch bei Indexfehler nicht zurück (siehe 4.4) — Risiko von Duplikaten/Stehenbleiben. `[RISIKO]`
- **Index-Mapping vs. `Event.Data`:** `buildIndexMapping` definiert u. a. `ReferenceTime`/`ReceptionTime`, `Event.Data()` liefert nur `Message` (`event.go`) — diese Zeitfelder werden aus `Data()` nicht befüllt. `[INFERENCE]` Zeitfilterung/Suche über diese Mapping-Felder ist daher aus dem Codepfad nicht zugesichert.
- **Retention-Minimum:** „Minimum 24h“ nicht erzwungen (siehe 4.5).

## 7. Review-Checkliste (C1, MS6-Änderungen)

Bei Änderungen an Ingest/Index/Query dieses Dokument und die folgenden Punkte gegen den Code prüfen:

1. **Lock-Reihenfolge/-Stil (`Engine.mu`, #13):** Halten `Index()`/`createIndex*`/`enforceRetention` konsistente Sperrkonventionen? Kein neues Upgrade/Downgrade ohne Begründung; RLock nicht über mutierende Pfade halten.
2. **Zeitbereichs-Index & Shard-Routing:** `IndexDuration`-Truncation, `[start,end)`-Semantik (`Contains`/`createIndexForReferenceTime`) und Shard-Hash (`fnv32a(DocID) % len(Shards)`, `Index.Shard`) unverändert oder Dokument + Migration bedacht.
3. **DocID/Zeitnormalisierung:** `Event.ID()` (msw = `ReferenceTime().UnixNano()`, lsw = `Sequence`), `ReferenceTime()`-Fallbacks und Parser-`timestamp`-Format (RFC3339-Erwartung) konsistent; kein ungeguardeter Panic-Pfad eingeführt/verschärft.
4. **Batch-Flush & Fehlerbehandlung:** `Batcher`-Trigger (`batchsize`/`batchtime`/`maxpending`), Reset/`errChan`-Verhalten bei Erfolg/Fehler, und Durabilitätsannahmen explizit.
5. **Retention:** `Expired`-Definition (`endTime + RetentionPeriod < now`), Check-Intervall, Löschpfad unter Lock, und ob ein Minimum durchgesetzt werden soll/muss (Code vs. Doku/Flag).
6. **Suchpfad & Reihenfolge:** Index-Iterationsrichtung, `DocIDs`-Sortierung, `bleve.NewQueryStringQuery` vs. `query/`-Paket (#32) — keine unbeabsichtigte Semantikänderung der Query-Sprache/Ergebnisordnung.
7. **Sicherheits-/Betriebsoberfläche:** Auth-Status der Query-Endpunkte (#14/#52) und pprof/expvar-Exposition (`-diag`) bei Änderungen mitdokumentiert; Verweis auf `docs/PRIVACY.md` aktuell.

---
*Erzeugt für Issue #79. Referenzsymbole u. a.: `main`/Flag-Defaults (`cmd/ekanited/main.go`), `NewEngine`/`Open`/`Index`/`Search`/`enforceRetention`/`RetentionCheckInterval` (`engine.go`), `NewBatcher`/`Start`/`C` (`engine.go`), `NewIndex`/`OpenIndex`/`Shard`/`Index.Shard`/`Index.Search`/`DeleteIndex`/`DocIDs.Less` (`index.go`), `Event.ID`/`Data` (`event.go`), `input.Event.ReferenceTime` (`input/event.go`), `NewCollector`/`TCPCollector.handleConnection` (`input/collector.go`), `NewServer`/`NewHTTPServer` (`server*.go`), `query.NewParser` (`query/parser.go`).*
