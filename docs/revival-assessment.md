# Ekanite – Assessment: Wiederbelebung und Standortbestimmung

**Stand:** 2026-11-01 · **Basis:** `master` @ `670bbbe` (2021-08-13, letzter Commit des Originals)
**Fork:** `schankst/ekanite` (privat, vom Original-Netzwerk abgetrennt) · **Upstream:** `ekanite/ekanite` (archiviert)

Legende der Evidenz:

- **[M]** gemessen in dieser Umgebung (Befehl/Ergebnis im Anhang reproduzierbar)
- **[A]** abgeleitet aus Code, Changelog oder Issue-Verlauf, nicht separat gemessen
- **[O]** offen – bewusst nicht geprüft, siehe §7

---

## 0. Kurzfazit

Der Code ist **nicht verrottet**: er baut und testet heute ohne eine einzige Änderung auf Go 1.26.10, mit 44 % Testanteil und einem End-to-End-Test, der den Daemon wirklich startet, Daten indexiert und abfragt. Was verrottet ist der **Betriebsrand**: CI auf einem EOL-Image, Releases von 2017, GOPATH-Build-Doku, keine Binaries, keine Container, keine Auth auf den Query-Ports.

Zwei Befunde verschieben die Prioritäten gegenüber der ursprünglichen Einschätzung:

1. **Der `-race`-Lauf ist heute kaputt** – nicht durch Ekanite-Code, sondern durch `etcd-io/bbolt@v1.3.3` (`checkptr`-Fehler). Die CI-Zeile `go test -race` würde auf jedem modernen Toolchain scheitern. Nach der bleve-v2-Migration läuft `-race` grün. **[M]**
2. **Der Ingest ist index-, nicht parser-limitiert.** Unter bleve v2 sind ~74 % der CPU in `bbolt`-Inserts während `scorch` einen Snapshot persistiert; das Framing (Delimiter + Leseschleife) liegt bei ~7 %. Die in Issue #80 versprochene 10×-Verbesserung durch einen besseren Delimiter ist mit dem aktuellen Stack **nicht** der Hebel. **[M]**

Der billigste sinnvolle Schritt ist daher: Fork aufräumen (CI/Release/Doku), bleve v2 einziehen (4 Imports), Batch-Default korrigieren, dann die Persistenz- und Suchpfade angehen. Feature-Ausbau (Tail, Zeitbereiche, API) ist Produktarbeit und nur bei konkretem Eigenbedarf sinnvoll.

---

## 1. Umgebung und Reproduzierbarkeit

| Punkt | Wert |
|---|---|
| Host | Linux 6.12.0-211.69.1.el10_2.x86_64, x86-64 |
| CPU | Intel Xeon (Skylake), **2 vCPU** sichtbar (`GOMAXPROCS set to 2`) |
| Go | go1.26.10 (Red Hat 1.26.10-1.el10_2) |
| Checkout | `670bbbe` = Upstream-`master` |
| Messdatum | 2026-10-31 / 2026-11-01 |
| Arbeitskopien | `/tmp/e2` (bleve-v2-Migration), `/tmp/bprobe*` (API-/Kompatibilitätsproben), `/tmp/genidx` (Legacy-Index-Fixture) |

Das geklonte Repo blieb während der Messungen unverändert; alle Experimente liefen auf Kopien bzw. in temporären Modulen. Diese Datei ist die einzige Änderung im Repo.

**Einschränkung:** Alle Durchsatz- und Speicherwerte sind **Einzelmessungen** auf einer 2-vCPU-VM ohne Wiederholung, ohne `fsync`-Tuning und ohne Kontrolle des Page-Cache. Die Größenordnungen sind belastbar, die Nachkommastellen nicht.

---

## 2. Zustand des Repositories

### 2.1 Größe und Struktur

| Paket | Prod-LOC | Test-LOC | Prod-Dateien | Rolle |
|---|---:|---:|---:|---|
| `.` (root) | 1.195 | 1.011 | 5 | Engine, Index/Shards, Batcher, TCP-Query-Server, HTTP-Query-Server |
| `cmd/ekanited` | 347 | 401 | 1 | Daemon, Flags, End-to-End-Test |
| `input` | 527 | 492 | 5 | TCP-/UDP-Collector, Delimiter, Parser-Fabrik |
| `parser` | 117 | 0 | 2 | RFC5424- und Watchguard-Parser (Regex) |
| `query` | 401 | 248 | 4 | **toter Code**: eigener Query-Parser/Lexer (siehe 2.3) |
| `rfc5424` | 214 | 186 | 2 | Frame-Reader für RFC5424 |
| `status` | 186 | 0 | 1 | expvar-Diagnostics |
| **Summe** | **2.987** | **2.338** | **20** | 30 `.go`-Dateien gesamt (20 Prod + 10 Test), 5.325 LOC |

Testanteil: **44 %** (2.338 von 5.325 LOC, 10 Testdateien). Für ein Projekt dieser Größe ungewöhnlich gut; der End-to-End-Test in `cmd/ekanited/system_test.go` startet den Daemon, schickt Syslog-Zeilen über TCP und prüft Suchergebnisse über beide Query-Schnittstellen.

### 2.2 Betriebsrand

| Bereich | Befund | Bewertung |
|---|---|---|
| CI | `.circleci/config.yml`, `circleci/golang:1.13` – Image seit Jahren EOL; Schritte: `go vet`, `gofmt.sh`, `go get -t -d`, `go test`, `go test -race` | **[M]** Der `-race`-Schritt scheitert heute (siehe 3.2) |
| Releases | letztes Tag `v1.3.0` vom **2017-11-27**; `CHANGELOG.md` führt 1.4.0 als „unreleased" (go.mod/Go-1.13-Refactor 2019) | README verweist auf Binaries von 2017 – 4 Jahre älter als der letzte Code-Stand |
| Build-Doku | `CONTRIBUTING.md` und `package.sh` sind GOPATH-Ära (`export GOPATH`, `go get`, `$GOPATH/bin`) | funktioniert mit Modulen so nicht mehr |
| Container | kein `Dockerfile`, kein `.github/` | Deployment nur per Hand-Build |
| Release-Skript | `package.sh` lädt per `curl` in die GitHub-Release-API, `go get -d ./...` | nicht reproduzierbar (kein Lockfile-Effekt, kein `goreleaser`) |
| Metadaten | `archived: true`, 770 Stars, 66 Forks, 8 Branches, 10 Tags | kein aktiver Fork im Netzwerk (jüngster Fork-Push 2023, 0 Stars) |
| Dieser Fork | `schankst/ekanite`, **privat**, vom Fork-Netzwerk gelöst (`fork: false`, `parent: none`); `origin` = Fork, `upstream` = Original | GitHub deaktiviert den Fork eines archivierten Repos („repository is disabled", 403); die Umstellung auf privat hat ihn aus dem Netz gelöst |

### 2.3 Toter Code

`query/` (401 Prod-LOC, 248 Test-LOC) wird von **keinem** Paket importiert (`grep -rn 'ekanite/query' --include=*.go .` → keine Treffer außerhalb des Pakets). Es ist der 2015 begonnene, nie verdrahtete Lucene-artige Parser aus Issue #8. Er wird getestet, aber nie ausgeführt – klassischer Fall von „grün, aber wirkungslos". Empfehlung: löschen, nicht fertigbauen (siehe 8.1).

### 2.4 Architektur in einem Absatz

`ekanited` startet einen Batcher (Kanal, Größe `-batchsize`, Timeout `-batchtime`), einen TCP-/UDP-Collector und eine Engine. Die Engine hält eine Liste von Indizes, je **Tag** (`20060102_1504` im Verzeichnisnamen, `endtime`-Datei daneben), jeder Index hat N **Shards** (Default 4), jeder Shard ist ein eigenes bleve-Index-Verzeichnis. Events landen per FNV-32a-Hash ihrer DocID in einem Shard; DocIDs sind 32-stellige Hex-Paare und werden lexikografisch als Zeitordnung benutzt. Retention löscht stündlich ganze abgelaufene Tagesindizes. Suche geht sequenziell über alle Indizes (älteste zuerst), pro Index über den bleve-`IndexAlias`, Trefferlimit 10.000, danach ein `GetInternal` pro Treffer für die Rohzeile.

---

## 3. Build- und Testverifikation

### 3.1 Unverändertes Repo auf Go 1.26.10

| Prüfung | Befehl | Ergebnis |
|---|---|---|
| Build | `go build ./...` | **ok** (13,3 s inkl. Modul-Downloads) **[M]** |
| Vet | `go vet ./...` | **clean**, keine Ausgabe **[M]** |
| Tests | `go test ./...` | **ok**, 8,3 s; `cmd/ekanited` 5,4 s (End-to-End) **[M]** |
| Race | `go test -race -count=1 ./...` | **FAIL** in `cmd/ekanited` (siehe 3.2) **[M]** |
| Statischer Build | `CGO_ENABLED=0 go build -o ekanited ./cmd/ekanited` (auf `/tmp/e2`) | **26.169.170 Byte, statisch gelinkt, kein CGO** **[M]** |

Bemerkenswert: `go.mod` deklariert `go 1.13`, der Code benutzt aber keine API, die neuer wäre. Es gibt **keinen** `godebug`-Eintrag, keine `replace`-Direktive, kein Build-Tag-Geflecht. Deshalb „fällt" der Build auch 5 Go-Versionen später nicht um.

### 3.2 Der `-race`-Lauf ist heute kaputt

```
fatal error: checkptr: converted pointer straddles multiple allocations
runtime.checkptrAlignment
  github.com/etcd-io/bbolt.(*Bucket).write   bbolt@v1.3.3/bucket.go:624
  ...
  github.com/ekanite/ekanite.(*Shard).Open  index.go:347
```

`bbolt` v1.3.3 (2019) benutzt `unsafe`-Pointer-Arithmetik, die Go's `checkptr`-Instrumentierung – aktiv bei `-race` – als Verstoß wertet. Das ist **kein** Fehler im Ekanite-Code, aber es bedeutet: der Race-Schritt der CI ist seit Jahren funktionslos, und niemand hat es gemerkt, weil die CI auf einem gepinnten 1.13-Image lief.

**Nach der bleve-v2-Migration** (`go.etcd.io/bbolt` v1.4):

```
go test -race -count=1 ./...   → alle Pakete ok (26,5 s)
```

Damit ist die Migration nicht nur „moderner", sondern **repariert einen heute kaputten Qualitäts-Gate**. **[M]**

### 3.3 Laufzeit-Smoke-Test (v2-Migrationskopie)

| Schritt | Ergebnis |
|---|---|
| Daemon starten (`-datadir`, `-tcp`, `-queryhttp`) | startet, Indizes werden angelegt |
| 40.000 RFC5424-Zeilen über TCP senden | 40.000 indiziert **[M]** |
| HTTP-Query `POST query=login` | `Listing 10000 results`, Treffer enthalten die gesendeten Zeilen **[M]** |
| Indexformat neu | `{"storage":"boltdb","index_type":"scorch"}` **[M]** |
| Ablage | 110 MB nach 40.000 Zeilen (Default-Batch, nach Merge), Details und Vergleich in 5.2 |

---

## 4. Abhängigkeiten und Sicherheit

### 4.1 Modulgraph

| Kennzahl | Original | nach bleve-v2-Migration |
|---|---:|---:|
| Module im Build (`go list -m all`) | **50** | **53** |
| Direkte Abhängigkeit mit Gewicht | `github.com/blevesearch/bleve` v0.8.1 (2019-08) | `github.com/blevesearch/bleve/v2` v2.6.1 |
| KV-Store | `github.com/etcd-io/bbolt` v1.3.3 (**Modulpfad tot**, heute `go.etcd.io/bbolt` v1.5.0) | `go.etcd.io/bbolt` v1.4.0 |
| CGO | nicht nötig | nicht nötig |

Der Rest des Graphen ist bleve-Transitiv (vellum, roaring, segment, gtreap, zapx, msgp …). Alles andere im `go.mod` (`bbolt`, `roaring`, `protobuf`, `gtreap`, `segment`, `go-porterstemmer`) ist als `// indirect` markiert und existiert nur wegen bleve – ein einziger schwerer Abhängigkeitsblock, der alles andere mitzieht. **[M]**

### 4.2 Schwachstellen (`govulncheck`, Originalstand)

```
Symbol Results:      No vulnerabilities found.
Package Results:     GO-2022-0493  x/sys/unix   (privilege reporting)
Module Results:      GO-2026-5024  x/sys/windows (integer overflow, Windows only)
                     GO-2022-0470  bleve (keine Zugriffskontrolle, "Fixed in: N/A")
Ihre Code-Pfade sind von 0 Schwachstellen betroffen.
```

Bewertung: **kein akutes Risiko im Originalstand**, aber drei Beobachtungen:

- `x/sys` von 2019 ist 6 Jahre alt; die Behebung ist ein `go get`.
- GO-2022-0470 („no access control in bleve") betrifft genau die fehlende Authentifizierung der Query-Ports – siehe 4.5. Sie ist nicht „im Code erreichbar", sondern eine Design-Eigenschaft.
- Nach der v2-Migration verschwinden `etcd-io/bbolt` und das alte `bleve`-Modul aus dem Graphen. **[M]**

### 4.3 Ist die bleve-Migration wirklich trivial? (verifiziert, nicht angenommen)

Die gesamte bleve-Nutzung von Ekanite steht in `index.go`:

| API | Verwendung | Verhalten unter v2.6.1 |
|---|---|---|
| `bleve.New(path, mapping)` | Shard anlegen | kompiliert unverändert; erzeugt jetzt **scorch** (Default `IndexType`) |
| `bleve.Open(path)` | Shard öffnen | unverändert |
| `bleve.NewIndexAlias()`, `Add`, `Search` | Suche über Shards | unverändert |
| `bleve.NewQueryStringQuery`, `NewSearchRequest` | Suche | unverändert |
| `batch.Index`, `batch.SetInternal`, `idx.Batch` | Schreiben | unverändert |
| `idx.GetInternal`, `DocCount`, `NewBatch` | Lesen | unverändert |
| `mapping.IndexMappingImpl`, `AddCustomTokenizer/Analyzer`, `NewTextFieldMapping`, `NewDateTimeFieldMapping`, `IncludeInAll`, `IncludeTermVectors` | Mapping/Analyzer | unverändert, nur Importpfade (`.../v2/...`) |
| `analysis/tokenizer/regexp`, `analysis/analyzer/custom` | Analyzer-Bausteine | unverändert, nur Importpfade |

**Diff der Migration:** 4 Importzeilen in `index.go` plus `go.mod`. Ergebnis: Build ok, `go test ./...` ok, `go test -race ./...` ok, End-to-End-Lauf ok. **[M]**

### 4.4 Bestehende Indizes – geht das ohne Reindex?

Explizit geprüft, weil das die Migration teuer machen würde:

1. Legacy-Index mit dem **gepinnten** bleve v0.8.1 und exakt dem Ekanite-Mapping erzeugt (`/tmp/genidx`), `index_meta.json` = `{"storage":"boltdb","index_type":"upside_down"}`.
2. Mit bleve **v2.6.1** geöffnet: `DocCount` = 1, Suche nach `sshd` = 1 Treffer, `GetInternal("0001")` liefert die Rohzeile.
3. Zusätzlich in den Legacy-Index **geschrieben** (Batch + `Index`), danach Suche nach dem neuen Term: 1 Treffer.

**Ergebnis:** v2 kann `upside_down`-Indizes lesen, durchsuchen **und** weiterschreiben. Ein Zwangs-Reindex ist nicht nötig; alte Tagesindizes laufen aus der Retention heraus, neue entstehen als scorch. **[M]**

Einschränkung: getestet mit einem Index aus 1 Dokument und dem Ekanite-Mapping. Ein Langzeittest mit 10⁶ Dokumenten über eine Retention-Periode steht aus. **[O]**

### 4.5 Sicherheitsbefunde am Dienst

| # | Befund | Schwere | Evidenz |
|---|---|---|---|
| S1 | `-queryhttp` und `-query` (Telnet) haben **keine Authentifizierung, kein TLS, kein Rate-Limit, kein Ergebnislimit unterhalb 10.000** | hoch, sobald nicht-loopback | `server_http.go`, `server.go`; Defaults `localhost:9950`/`:8080`, README-Beispiele und Issue-Logs zeigen `0.0.0.0:8080` |
| S2 | `-diag` exponiert `expvar` **und `net/http/pprof`** unauthentifiziert | mittel (Heap-/CPU-Profile = Inhalte des Logspeichers) | `cmd/ekanited/main.go`, `status/service.go` |
| S3 | `-tcp` ist Klartext-Syslog; TLS nur optional über `-tlspem`/`-tlskey` | mittel | Flags; Client-Zertifikatsprüfung nicht verifiziert **[O]** |
| S4 | `bleve` GO-2022-0470: „no access control" – deckt sich mit S1 | mittel, Design | govulncheck |
| S5 | Query-Parameter nur per **POST**; `GET ?query=…` wird ignoriert (liefert das Formular) | niedrig, Usability | `server_http.go:60-95` – beim Testen real aufgetreten |
| S6 | Angezeigte Suchdauer ist falsch: `dur` wird zwischen `Search()`-Aufruf und dem Drainieren des Ergebnis-Kanals gemessen, also nur die Kanal-Erzeugung | niedrig, irreführend | `server_http.go:88-100` |
| S7 | XSS: Logzeilen gehen durch `html/template` → automatisch escaped | kein Befund | `server_http.go:5,199-201` |

Kurz: das Bedrohungsmodell ist „vertrauenswürdiges LAN". Für alles andere fehlt die gesamte Schicht (Auth, TLS, Limits, Audit). Wer den Dienst heute ins Netz stellt, veröffentlicht seine Logs.

---

## 5. Performance

### 5.1 Methodik

- Last: 100.000 identische RFC5424-Zeilen à **91 Byte** (`<134>1 … connection closed from 10.1.2.3 port 4242`) über **eine** TCP-Verbindung, geschrieben mit `bufio` 64 KiB.
- Konfiguration: 4 Shards (Default), `-batchtime 200`, sonst Defaults; verglichen wird `-batchsize` 300 (Default) vs. 5000.
- Messpunkte: (a) Zeit bis der Sender alle Bytes losgeworden ist (TCP-Gegendruck), (b) Zeit bis `engine.eventsIndexed == n` via `-diag`/expvar, (c) Plattenverbrauch nach Merge-Settling (~45 s Leerlauf).
- Zusätzlich: Mikrobenchmark des Delimiters (`go test -bench`, Framing ohne Socket/Index) und ein CPU-Profil über `-cpuprof` (32,9 s Laufzeit, 38,6 s Samples ≈ 117 % wegen 2 vCPU).
- Vergleichsbasis: identischer Lauf mit dem **unveränderten** Original-Build (`/tmp/ekanited-old`, bleve 0.8.1).

### 5.2 Ergebnisse (100k Zeilen, 91 B)

| Stack | `-batchsize` | Sendezeit | indiziert (end-to-end) | Platte | pro Zeile |
|---|---:|---:|---:|---:|---:|
| bleve **0.8.1** (Original) | 300 | 7,0 s (14.302/s) | **10,0 s → 9.965 Zeilen/s** | 502 MB | 5,1 KB |
| bleve **0.8.1** (Original) | 5000 | 15,2 s (6.589/s) | 18,7 s → 5.346 Zeilen/s | 502 MB | 5,1 KB |
| bleve **v2.6.1** | 300 (Default) | **76,1 s (1.314/s)** | **>196 s, 95.700/100.000 indiziert** | **2,3 GB** | 23 KB |
| bleve **v2.6.1** | 5000 | 6,6 s (15.212/s) | **18,0 s → 5.560 Zeilen/s** | **328 MB** | 3,3 KB |
| Referenz ohne Last | – | – | – | 1,4 MB (nur 1.000 Zeilen) | – |

Alle Werte **[M]**, Einzelmessungen, 2 vCPU.

Drei Aussagen, die daraus folgen:

1. **Der Default `-batchsize 300` ist unter v2 pathogen**: 1,3 k statt 15 k Zeilen/s Sendedurchsatz und **2,3 GB statt 328 MB** für dieselben 100.000 Zeilen – bei identischem Stack Faktor **7** bei der Platte und Faktor **~11** beim Durchsatz (1.314 vs. 15.212 Zeilen/s beim Senden, ~490 vs. 5.560 Zeilen/s beim Indizieren). Mit dem alten bleve ist derselbe Default unauffällig (10 s, 502 MB). Die Kombination „neuer Stack + alter Default" ist die schlechteste aller vier Varianten.
2. **v2 mit großem Batch ist beim Durchsatz gleichauf** mit dem alten Stack (~5,5 k/s), aber **kleiner auf der Platte** (328 MB vs. 502 MB).
3. Der alte Stack ist bei kleinem Batch **schneller** (10 k/s), bezahlt das aber mit dem kaputten `-race`-Gate, dem toten `bbolt`-Pfad und drei Modul-Advisories.

### 5.3 CPU-Profil (v2, `-batchsize 5000`, 100k Zeilen)

| Funktion | flat | cum |
|---|---:|---:|
| `runtime.memmove` | 68,4 % | 68,4 % |
| `bbolt.(*node).put` | 0,4 % | **73,3 %** |
| `util.(*BoltBucketImpl).Put` | 0,03 % | 77,5 % |
| `scorch.prepareBoltSnapshot` | 0,03 % | 78,4 % |
| `scorch.(*Scorch).persistSnapshotDirect` | 0 % | 81,0 % |
| `scorch.(*Scorch).persisterLoop` | 0 % | **81,7 %** |
| `input.(*TCPCollector).handleConnection` | 0 % | **6,9 %** |
| `regexp.(*Regexp).tryBacktrack` | 0,7 % | 1,5 % |
| restliche Knoten (<0,19 s) / Runtime | – | ~14 % |

Call-Kette des Hotspots: `persisterLoop → persistSnapshot → persistSnapshotDirect → prepareBoltSnapshot → BoltBucketImpl.Put → bbolt.Bucket.Put → node.put → typedslicecopy → memmove`.

### 5.4 Ursachenanalyse

**(a) Snapshot-Kopien statt inkrementeller Writes.** `scorch` persistiert pro Indexierungs-Batch einen Snapshot und kopiert dabei den internen KV-Store (Dokumentablage in `root.bolt`) in einen neuen Bucket. Die Kosten sind **O(Dokumente im Shard) pro Persist**, also über den Lauf **O(n² / batch)**. Bei 100k Zeilen, 4 Shards und Batch 300 sind das ~334 Persists pro Shard; bei Batch 5000 nur ~20. Das erklärt sowohl die CPU-Verteilung (73 % in `bbolt`-Inserts, davon der Löwenanteil `memmove`) als auch das Dateiwachstum: `root.bolt` wuchs im Default-Lauf auf **905 MB pro Shard** für ~25.000 Dokumente à ~91 Byte.

**(b) Der Batch wird über Shards aufgeteilt.** `Engine.Index` splittet eine Charge auf die Shards und indexiert jeden Sub-Batch separat; jeder Sub-Batch löst einen eigenen Persist aus. Effektiv multipliziert die Shard-Zahl die Snapshot-Kosten. Die Shards waren 2015 ein Mittel gegen die Concurrency-Grenzen von `upside_down`; unter scorch (das eigene Hintergrund-Merges hat) ist der Nutzen fraglich.

**(c) Framing ist billig.** Der Delimiter-Benchmark (nur `Push`, ohne Socket):

| Zeilen | ns/op | MB/s | Allocs/Zeile | Müll/Zeile |
|---:|---:|---:|---:|---:|
| 10.000 | 42,4 ms | 21,0 | 4 | 289 B |
| 100.000 | 398 ms | 22,4 | 4 | 289 B |

Umgerechnet: **~236.000 Zeilen/s** Durchsatz für reines Framing, linear skalierend (kein O(n²) im Delimiter). Im Profil bleibt die komplette Leseschleife (`handleConnection`, inkl. `ReadByte`, `SetReadDeadline` pro Byte, `stats.Add`, Parsing, Kanal-Send) bei **6,9 %** der CPU.

**(d) Die Leseschleife selbst ist unnötig teuer, aber derzeit nicht der Engpass.** `input/collector.go` macht pro **Byte**: `reader.ReadByte()`, `conn.SetReadDeadline()`, zwei `stats.Add()` und ein `delimiter.Push()` mit `regexp.FindIndex` über den gepufferten Rest. Das ist ein Syscall pro Byte plus zwei Map-Updates; bemerkbar machen wird sich das erst, wenn die Persistenz nicht mehr 90 % der Zeit frisst (rechnerisch ab ~50k Zeilen/s). `bufio.Reader.ReadSlice`/`ReadBytes` mit einem Byte-Scanner statt Regex ist trotzdem die richtige Zielarchitektur.

### 5.5 Suchpfad

`Engine.Search` läuft sequenziell über **alle** Indizes (älteste zuerst), pro Index über den Alias aller Shards, holt `maxSearchHitSize = 10000` Treffer-IDs, ruft dann **pro Treffer** `GetInternal` auf, sortiert die DocIDs über `strconv.ParseUint` auf Hex-Präfixen und schickt die Rohzeilen über einen ungepufferten Kanal (Puffer 1). Es gibt:

- **keinen** Zeitbereichsfilter – obwohl die Daten tagesweise getrennt sind und man ganze Indizes überspringen könnte (Issue #13),
- **keine** Paginierung, nur die harte 10.000er-Grenze (Issue #15),
- **kein** `context` – eine Suche ist nicht abbrechbar, ein langsamer Konsument blockiert die Goroutine,
- keine Relevanzsortierung (DocIDs = Zeitordnung, bewusst).

Ein Such-Latenz-Benchmark wurde **nicht** erhoben (Testdaten nach den Messungen gelöscht). **[O]**

### 5.6 Korrektur einer früheren Annahme

Eine frühere Einschätzung (vor den Messungen) hielt den Input-Pfad (#80) für den größten Einzelhebel (10×). Das war **nicht belegt** und ist durch die Messungen widerlegt: bei v2/batch5000 sind es 6,9 % CPU; selbst ein perfekter Delimiter bringt dort ~7 %, nicht 10×. Die 10×-Behauptung aus #80 stammt aus 2017 mit dem alten Stack und ist mit v2 nicht reproduzierbar. Auch die frühere Angabe „~25k Zeilen/s Obergrenze" für den Delimiter war ein Rechenfehler; korrekt sind ~236k/s (siehe Tabelle 5.4c).

Der Hebel liegt in der **Persistenz** (Batch-/Shard-Konfiguration, Snapshot-Kosten), nicht im Framing.

---

## 6. Issues (33 insgesamt: 18 offen, 15 geschlossen)

Stand: alle Issues des Originals, gelesen am 2026-11-01. Kategorien: **BUG** (bestätigter Defekt), **PERF**, **FEAT** (Feature-Wunsch), **SUP** (Support/Frage/Rauschen), **META**.

| # | Titel (gekürzt) | Status | Kat. | Befund / Maßnahme |
|---:|---|---|---|---|
| 1 | Tokenise digits? (`ssh` findet `sshd` nicht) | offen | BUG | Analyzer-Tokenizer `[^\W_]+` matcht keine Präfixe. Fix: Edge-N-Gramm-Analyzer oder `keyword`-Feld, ca. 1 Stunde. |
| 2 | Live-Tail-Feature | offen | FEAT | Meistgewünscht (siehe #88). Braucht Streaming-Endpoint + Cursor-Semantik. |
| 3 | E2E-Tests bei `DefaultNumShards = 1` | closed | SUP | 2015 erledigt. |
| 4 | „Index source of parsed lines is actually entire line" | offen | BUG | Semantik: Rohzeile statt Message wird als `Source` gespeichert; Syslog-Header landet in den Suchergebnissen. Betrifft die Anzeige, nicht den Index. |
| 5 | Unparsbare Zeilen als Source indizieren | offen | BUG | Heute: unparsbare Zeilen werden verworfen (`rfc5424Unparsed`). Das ist die Ursache des „nichts in den Logs"-Falls aus #98. |
| 6 | BoltDB-Füllgrad erhöhen | closed | SUP | Vom Autor als Missverständnis geschlossen. |
| 7 | Retention `7d` nicht parsebar | offen | BUG | `time.ParseDuration` kennt kein `d`. Trivial (Suffix-Mapping), ~30 min. |
| 8 | Lucene-Query-Sprache verdrahten | offen | FEAT | Parser liegt als toter Code in `query/`. `QueryStringQuery` deckt das Nötige ab → **Code löschen**, Issue schließen. |
| 9 | Feldsuche `apache.statuscode: 404` | offen | FEAT | Von bleve bereits unterstützt (`field:wert`), nur nicht dokumentiert; das Ekanite-Mapping indiziert allerdings nur `Message` (+ Zeiten). |
| 10 | Regex-Suche | offen | FEAT | In bleve als `regexp`-Query vorhanden; nicht exponiert. Klein. |
| 11 | SSL-Transport für Logs | closed | FEAT | 2015 per `-tlspem`/`-tlskey` umgesetzt. |
| 12 | LevelDB-Backend testen | offen | PERF | Durch scorch überholt; C-Abhängigkeit wäre ein Rückschritt. Schließen. |
| 13 | Zeitbereichs-Queries | offen | PERF | **Größter Suchgewinn.** Tagesindizes existieren; es fehlt Pruning + Range-Filter. 1–2 Tage. |
| 14 | Apache Common Format automatisch parsen | offen | FEAT | Eigener Parser (Parser-Fabrik existiert bereits, inkl. Watchguard-Beispiel). 1 Tag. |
| 15 | CLI-Client mit Paging/Diagnose | offen | FEAT | Fehlt komplett; 10.000er-Hardlimit macht ohne Paging wenig Sinn. |
| 16 | Regex-Parsing durch State Machine ersetzen | offen | PERF | Regex insgesamt 1,5 % CPU im Profil – **nicht** der Engpass. Niedrige Priorität. |
| 17 | „no tokenizer type 'regexp' registered" | closed | BUG | 2015 per Commit gefixt; Klasse des Fehlers (Analyzer wird per Typname referenziert) existiert weiter: ein Index lässt sich nur öffnen, wenn das Tokenizer-Paket importiert ist. Beim Probe-Lauf real reproduziert. |
| 18 | Sammelissue Suchqualität/Disk (84k Logs = 1 GB) | closed | SUP | Als Bleve/Bolt-Thema geschlossen. Die Disk-Beobachtung ist **weiterhin gültig** (5.2). |
| 19 | Stemming untersuchen | offen | FEAT | 1 Zeile Analyzer-Konfiguration (`porter`-Filter). Nur sinnvoll für Fließtext, nicht für Logs. Niedrig. |
| 23 | „README-Build-Anleitung falsch" | closed | SUP | War Nutzerfehler; die Anleitung ist inzwischen trotzdem veraltet (GOPATH). |
| 35 | Hohe CPU-Last im Dauerbetrieb | closed | PERF | **Ohne Lösung geschlossen.** Die Messungen in 5.3/5.4 liefern die Erklärung: Snapshot-Persistenz + Default-Batch. Zusätzlich: Suche über alle Indizes ohne Zeitfilter. |
| 58 | syslog-ng-Beispielkonfiguration | closed | SUP | Ohne Reproduktion geschlossen; README-Vorlage ist plausibel. |
| 67 | Stundenindizes zu fein | closed | FEAT | Auf Tagesindizes umgestellt. |
| 69 | S3-Backup/Restore | offen | FEAT | Architektonisch schwierig (konsistenter Snapshot über Bleve). **Nicht bauen** – Volume-Snapshots/Restic dokumentieren. |
| 70 | Beispiel-Client | closed | SUP | Auf README verwiesen. |
| 75 | Geteilte Parser → Fehlparsungen | closed | BUG | 1.2.1 gefixt (Parser pro Verbindung). |
| 79 | Korrupte Suche bei Shards > 10 | closed | BUG | 1.3.0: Shard-Namen vierstellig, `Close()` beim Beenden. Beides im Code vorhanden. |
| 80 | 10× Durchsatz mit `bufio.ReadString` | offen | PERF | Codebefund bestätigt (`Push` pro Byte mit Regex über den Puffer). **Aber:** mit v2 nur ~7 % CPU-Anteil → Priorität gesenkt, Architektur trotzdem richtig (5.4d). |
| 85 | Client-Beispiele / offizielle Client-Lib | offen | FEAT | Kernschwäche: nur Syslog-Ingest, kein API-Vertrag. Beispiele + JSON-API wären der Einstieg. |
| 88 | `tail -f`-Äquivalent mit Suchfilter | offen | FEAT | Duplikat von #2, drei unabhängige Stimmen (2018, 2019, 2020). |
| 92 | Windows-Support | closed | SUP | Nie getestet; Code ist plattformneutral (kein `syscall`, kein CGO) → sollte bauen. **[O]** |
| 94 | „Wird das Projekt gepflegt?" | closed | META | Antwort 2019: „Ich fixe Bugs, keine neuen Features." 2022 archiviert. |
| 98 | Diagnose fehlender Logs | closed | META | Abschiedsnotiz des Autors + Archivierung. Enthält aber den echten UX-Mangel: `input: {}` im expvar ist nicht selbsterklärend, und unparsbare Zeilen sind unsichtbar (#5). |

### 6.1 Verteilung

| Kategorie | offen | closed |
|---|---:|---:|
| BUG (bestätigt) | 4 (#1, #4, #5, #7) | 3 (#17, #75, #79) |
| PERF | 4 (#12, #13, #16, #80) | 1 (#35) |
| FEAT | 10 (#2, #8, #9, #10, #14, #15, #19, #69, #85, #88) | 2 (#11, #67) |
| SUP/META | – | 9 (#3, #6, #18, #23, #58, #70, #92, #94, #98) |

### 6.2 Muster im Issue-Verlauf

- **Die offenen Issues sind fast alle Produktwünsche, nicht Defekte**: 10 von 18 offenen sind Features (Tail, CLI, Zeitbereiche, Formate, Client-Lib, Stemming). Vier sind kleine, klar umrissene Bugs (#1, #4, #5, #7) mit zusammen ~1 Tag Aufwand.
- **Zwei Bugs wurden „erledigt" geschlossen, obwohl sie es nicht sind**: #35 (hohe CPU) ohne Ursache, #18 (Speicherverbrauch) als Bleve-Thema abgewälzt. Beide sind mit den Messungen aus §5 adressierbar.
- **Der Autor hat konsistent auf „keine neuen Features" verwiesen** (#94, #98). Wer das Projekt weiterführt, übernimmt genau diese Erwartungshaltung – oder muss sie explizit brechen.
- **Fork-Interesse ist erloschen**: 66 Forks, keiner mit Aktivität nach 2023, keiner mit Sternen. Es gibt keinen konkurrierenden Wiederbelebungsversuch, aber auch keine Community, die wartet.

---

## 7. Risiken und Grenzen dieser Aussagen

| Punkt | Status | Anmerkung |
|---|---|---|
| Durchsatz/Speicher | **[M]**, aber Einzelmessungen | 2 vCPU, kein `fsync`-Tuning, Page-Cache nicht kontrolliert; Wiederholung mit 3 Läufen und Median empfohlen, bevor Zahlen in ein README wandern |
| Suchlatenz | **[O]** | Testdaten gelöscht; Messung mit 1 Mio. Dokumenten und typischen Queries fehlt |
| Verhalten > 1 Mio. Dokumente/Index | **[O]** | Der O(n²)-Verdacht aus 5.4a ist aus dem Profil abgeleitet, nicht über eine Retention-Periode verifiziert |
| Legacy-Index-Kompatibilität | **[M]** mit 1 Dokument | Nur `upside_down` + Ekanite-Mapping geprüft |
| UDP-Pfad, Watchguard-Parser | **[O]** | Code gelesen, nicht ausgeführt |
| Crash-Recovery (SIGKILL mitten im Batch) | **[O]** | `Engine.Close()` beim Shutdown existiert seit 1.3.0; hartes Kill nicht getestet |
| Windows | **[O]** | keine `syscall`/CGO-Nutzung im Code, sollte bauen |
| Retention-Laufzeit bei vielen Indizes | **[O]** | stündlicher Sweep löscht ganze Verzeichnisse; Laufzeit unkritisch erwartet |
| Upstream-Sync | gegeben | Original ist archiviert; keine Patches mehr zu erwarten. `upstream` bleibt nur als Referenz. |

---

## 8. Empfehlung und Phasenplan

**Empfehlung: weiterführen – aber als scharf geschnittenes Werkzeug, nicht als Log-Plattform.**

Bedingungen, unter denen es sich lohnt:

1. Der Nutzen ist „ein statisches Binary, Syslog rein, Volltextsuche raus, Retention nach Tagen" – ohne JVM, ohne Cluster, ohne externe Dienste.
2. Du akzeptierst, dass Ingest und Speicherbedarf **eine Größenordnung** über VictoriaLogs/Loki/Quickwit liegen und das auch nach Optimierung bleiben wird.
3. Du willst die vier kleinen Bugs und die drei Suchfunktionen (Zeitbereich, Paging, Tail) tatsächlich bauen – sonst bleibt es ein Archiv mit Bedienungsfrust.

Phasen (Aufwand = reine Arbeit, ohne Review/Testzyklen):

| Phase | Inhalt | Aufwand | Erwarteter Effekt |
|---|---|---|---|
| **P0** | Modulpfad auf `github.com/schankst/ekanite`; GitHub Actions (Matrix 1.24/stable, `-race -shuffle=on`, golangci-lint v2, govulncheck, `go mod tidy`-Check); GoReleaser + GHCR-Image; README/CONTRIBUTING auf Module umstellen; `package.sh` löschen | 1–2 Tage | reproduzierbare Builds, `-race` wieder scharf, Binaries für linux/darwin/arm64 |
| **P1** | bleve v2.6.1 (4 Imports) + `go mod tidy`; `-batchsize`-Default 300 → 5000; `go 1.24`-Direktive; toten `query/`-Code löschen; `slog` statt `log.Logger`; `any`/`slices`/`maps` per `go fix` + `modernize` | 2–3 Tage | **`-race` grün**, CVEs raus, 7× weniger Platte, ~11× Ingest gegenüber dem alten Default, −650 LOC |
| **P2** | Index-Persistenz: Shard-Zahl evaluieren (1 vs 4), Batch-/Snapshot-Verhalten messen, ggf. Doc-Store aus dem Index lösen (Rohzeile in eigenes KV statt `SetInternal` in bleve) | 3–5 Tage | Ziel: >20k Zeilen/s und <1 KB/Zeile; ohne diesen Schritt bleibt P3 Kosmetik |
| **P3** | Kleine Bugs: #7 (`7d`), #1 (Präfixsuche), #5 (unparsbare Zeilen indizieren), #4 (Header aus der Anzeige) | 1–2 Tage | Issue-Liste halbiert, #98-Klasse von Supportfällen verschwindet |
| **P4** | Zeitbereichs-Queries mit Index-Pruning (#13), Paging + JSON-API (#15), Tail-Streaming (#2/#88), TLS+Auth auf Query-Ports (S1), Client-Beispiele (#85) | 2–3 Wochen | aus „Demo" wird benutzbar |
| **P5** | Härtung: `context` durch Collector/Engine/Search, Fuzz-Tests für Delimiter/Parser, Benchmarks im CI, `go test -race` als Pflicht-Gate | 3–5 Tage | Regressionen werden sichtbar |

### 8.1 Was man **nicht** bauen sollte

| Issue | Warum nicht |
|---|---|
| #8 eigener Lucene-Parser | `QueryStringQuery` deckt Feldsuche, Wildcards, Ranges, Regex. 401 LOC toter Code löschen statt fertigstellen. |
| #12 LevelDB | C-Abhängigkeit, kein Vorteil gegenüber scorch. |
| #69 S3-Backup/Restore | Konsistente Snapshots über Bleve hinweg sind der harte Teil (siehe Autor-Kommentar). Volume-Snapshot/Restic dokumentieren. |
| #94 pluggable Storage | Verdoppelt die Testmatrix für einen Nutzen, den niemand belegt hat. |
| #16 State-Machine-Parser | 1,5 % CPU im Profil. Erst nach P2 relevant. |
| #19 Stemming | Für Logzeilen schädlich (Host-/Feldnamen werden verfälscht). |

---

## 9. Anhang: Reproduktion

```bash
# 1. Baseline (unverändertes Repo)
go build ./... && go vet ./... && go test ./...
go test -race -count=1 ./...          # erwartet: FAIL (checkptr in bbolt 1.3.3)

# 2. bleve-v2-Migration auf einer Kopie
cp -r . /tmp/e2 && cd /tmp/e2
sed -i 's|github.com/blevesearch/bleve"|github.com/blevesearch/bleve/v2"|' index.go
sed -i -E 's|"github.com/blevesearch/bleve/(analysis/analyzer/custom\|analysis/tokenizer/regexp\|mapping)"|"github.com/blevesearch/bleve/v2/\1"|' index.go
go get github.com/blevesearch/bleve/v2@latest && sed -i '/blevesearch\/bleve v0.8.1/d' go.mod && go mod tidy
go build ./... && go test ./... && go test -race -count=1 ./...   # erwartet: alles grün

# 3. Delimiter-Benchmark (nur Framing)
cat > input/bench_test.go <<'EOF'
package input
import ("fmt";"strings";"testing")
func benchLines(n int) []byte {
    line := "<134>1 2026-10-31T10:00:00Z host1 app1 123 - - connection closed from 10.1.2.3 port 4242\n"
    var b strings.Builder
    for i := 0; i < n; i++ { b.WriteString(line) }
    return []byte(b.String())
}
func BenchmarkDelimiterPush(b *testing.B) {
    for _, n := range []int{10000, 100000} {
        data := benchLines(n)
        b.Run(fmt.Sprintf("lines=%d", n), func(b *testing.B) {
            b.SetBytes(int64(len(data)))
            for i := 0; i < b.N; i++ {
                d := NewSyslogDelimiter(msgBufSize); count := 0
                for _, c := range data { if _, ok := d.Push(c); ok { count++ } }
                if count != n-1 { b.Fatalf("got %d", count) }
            }
        })
    }
}
EOF
go test ./input -run xxx -bench BenchmarkDelimiterPush -benchtime 3x -benchmem

# 4. End-to-End-Last + Profil
go build -o /tmp/ekanited ./cmd/ekanited
/tmp/ekanited -datadir /tmp/ekdata -tcp 127.0.0.1:15516 -udp "" -query "" \
    -queryhttp "" -diag 127.0.0.1:19951 -batchtime 200 -batchsize 5000 -cpuprof /tmp/cpu.pprof &
# 100k Zeilen senden (Sender: bufio + Warten auf engine.eventsIndexed==n via /debug/vars)
kill -INT %1 && go tool pprof -top -cum /tmp/ekanited /tmp/cpu.pprof
du -sh /tmp/ekdata   # nach ~45 s Leerlauf (Merge-Settling)

# 5. Legacy-Index-Kompatibilität
#    /tmp/genidx: Modul mit bleve v0.8.1, erzeugt Index im Ekanite-Mapping
#    /tmp/bprobe: Modul mit bleve/v2, öffnet /tmp/oldidx/0000, sucht, schreibt
```

**Messumgebung für alle Zahlen:** siehe §1. Rohdaten (Profile, Benchmarks) lagen unter `/tmp` und sind nach Abschluss aufgeräumt; die Befehle oben erzeugen sie neu.
