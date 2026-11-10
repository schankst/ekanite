# Kapazität & Dimensionierung (Issue #77)

**Zweck:** reproduzierbare Richtwerte für Betrieb und Dimensionierung („welche Last schafft X RAM/CPU?“) und eine Grundlage für Performance-Vergleiche (u. a. die Engine-Migration, #118).

## Benchmarks (Quelle der Zahlen)

| Datei | Benchmarks |
|---|---|
| `input/bench_test.go` | `BenchmarkDelimiterOctet`, `BenchmarkDelimiterSyslog`, `BenchmarkParserRFC5424`, `BenchmarkParserBSD`, `BenchmarkParserFortiOS`, `BenchmarkIngestPath` |
| `bench_test.go` | `BenchmarkBatchIndexing` (100k Events, Produktions-Batchgröße/-Shards, **mit geparsten Feldern**) |

```sh
# lokal (stabile Mikro-/Parser-Werte):
go test -run '^$' -bench . -benchmem ./...

# 100k-Indexierung (gebunden, ein Messpunkt):
go test -run '^$' -bench BenchmarkBatchIndexing -benchmem -benchtime=1x ./
```

In der CI läuft der Job **`Benchmark (informativ)`** (nicht blockierend, `-benchtime=1x`, Artefakt `benchmarks`).

## Referenzmessung

Umgebung: linux/amd64, **2 vCPU** (Intel Xeon Skylake), Go 1.26; Engine mit `-numshards 4`, `-batchsize 300` (Produktions-Defaults). Die Indexierungs-Events werden mit dem **echten Parser** erzeugt, damit die Dokumentform (und damit die Indexgröße) der Produktion entspricht.

| Benchmark | ns/op | Durchsatz | B/op | allocs/op |
|---|---:|---:|---:|---:|
| `DelimiterOctet` (100 Frames) | 46.257 | 294 MB/s | 14.904 | 106 |
| `DelimiterSyslog` (100 Nachrichten) | 634.699 | 21 MB/s | 81.767 | 203 |
| `ParserRFC5424` (1 Zeile) | 7.086 | 18,6 MB/s | 2.542 | 30 |
| `ParserBSD` (1 Zeile) | 1.759 | 25,0 MB/s | 882 | 11 |
| `ParserFortiOS` (1 Zeile) | 3.967 | 74,4 MB/s | 4.528 | 42 |
| `IngestPath` (Parse+Event) | 7.304 | — | 2.784 | 32 |
| `BatchIndexing` (100k Events, 11 Felder) | ≈4,38 · 10¹⁰ (43,8 s) | **≈2.290 Events/s** | 27,8 GB | 280 Mio |

Pro Nachricht: Octet-Framing ≈ 0,46 µs · PRI/LF-Framing ≈ 6,3 µs · RFC5424-Parse ≈ 7,1 µs · **Indexierung ≈ 437 µs**.

## Kapazitätstabelle (Richtwerte)

Indexierung skaliert mit Kernen (Shards werden parallel indiziert), bis Disk-I/O begrenzt:

| CPU-Kerne | Events/s (Indexierung, 11 Felder) | Anmerkung |
|---:|---:|---|
| 2 | **~2.300** | gemessen (Referenz) |
| 4 | ~4.600 | Annahme: lineare Skalierung |
| 8 | ~9.200 | Annahme; ab hier Disk-I/O prüfen |

## Speicher

### Disk: die Größe hängt an der **Feldanzahl**, nicht an der Zeilenlänge

Gemessen (20k Events, 4 Shards, echte Parser):

| Format | Felder/Event | Disk/Event | Indexierzeit/20k |
|---|---:|---:|---:|
| BSD | 5 | **15,8 kB** | 3,8 s |
| RFC 5424 (mit SD) | 11 | **30,9 kB** | 7,7 s |
| FortiOS | 23 | **47,4 kB** | 12,7 s |
| RFC 5424, 100k Events | 11 | **28,6 kB** | 43,8 s |

⇒ **~1,5–2 kB pro Feld und Event**; die Rohzeile (~100 B) ist praktisch bedeutungslos. Ein Feld mehr kostet mehr als die ganze Nachricht.

### Woher die Bytes kommen (Zerlegung, 20k Events)

| Konfiguration | Disk/Event | Differenz |
|---|---:|---:|
| A: Produktionsform (Message + 8 geparste Felder + Rohzeile) | 20,8 kB | — |
| C: nur `Message` | 5,8 kB | geparste Felder ≈ **15,1 kB** |
| F: `Message` + **ein** nicht gemapptes Feld (`timestamp`) | 10,6 kB | **+4,9 kB für ein Feld** |
| E: wie A, aber `Dynamic=false` | 16,7 kB | dynamischer Pfad ≈ **4,1 kB** |
| G: wie A, `Message` nicht im `_all` | 18,9 kB | `_all`-Duplikat ≈ **1,9 kB** |
| H: wie E, ohne `SetInternal` | 16,4 kB | Rohzeile ≈ **0,3 kB (≈1,4 %)** |

**Ursache:** Felder, die nicht explizit im Mapping stehen (`buildIndexMapping`, `index.go`) — u. a. `timestamp` und **alle** FortiOS-Felder (`devname`, `logid`, `srcip`, `dstip`, `action`, `level`, …) — laufen über bleves **dynamisches Mapping**: `StoreDynamic`, `IndexDynamic`, `DocValuesDynamic` sind default `true`, und das erzeugte Feldmapping hat zusätzlich `IncludeInAll` und `IncludeTermVectors` an. Jedes solche Feld wird also **gespeichert, invertiert, mit Term-Vektoren, DocValues und im `_all`** abgelegt.

### Layout

Es gibt **eine boltdb-Datei pro Shard**, buchstäblich `<index>/<shard>/store` (upsidedown v1.0.14 schreibt Dictionary, Postings, Backindex *und* die via `SetInternal` abgelegten Rohzeilen in denselben KV-Store; **keine** zap-Segmente). Die bbolt-Seiten-/B-Tree-Overhead ist der Sockelbetrag; deshalb **sinkt** Disk/Event mit dem Volumen (feldfreie Events: 16,8 kB @1k → 10,8 kB @100k).

### Betriebs-Richtwerte

- 1 Mio Events (11 Felder) ≈ **28,6 GB**; 24 h bei 100 Events/s (~8,6 Mio) ≈ **246 GB**.
- `-rawmode drop` (DSGVO #93/#95) entfernt nur die Rohzeile ⇒ **< 2 %** Ersparnis.
- **RAM:** Allokationsvolumen ≈ 278 KiB/Event (kumulativ, GC-bereinigt — **nicht** Peak-RSS). `-batchsize` (300) und `-maxpending` (1000) begrenzen den Puffer; Peak-RSS offen.

## Annahmen

- RFC 5424 **mit** strukturierten Daten und geparsten Feldern; bleve v1.0.14 (upsidedown + boltdb) mit dem Mapping aus `index.go` (#34); lokale SSD; kein TLS; keine Vektor-/Geo-Felder.
- Änderungen an Retention, Shard-Zahl, Batchgröße oder **Mapping** verschieben die Werte (letzteres am stärksten).

## Regressionsregel

> Ein Benchmark-Ergebnis, das **> 10 %** schlechter ist als die Referenz, ist ein Review-Blocker (Issue #39, #77).

Der CI-Job ist **informativ** (ein Messpunkt, geteilte Runner = verrauscht). Der belastbare Vergleich erfolgt lokal mit `benchstat` auf derselben Maschine:

```sh
go test -run '^$' -bench . -count=10 ./... > new.txt   # Baseline: old.txt
benchstat old.txt new.txt
```

## Bekannte Engpässe

- **Index-Blähung durch dynamisches Mapping** (siehe oben): ~72 % der Bytes stammen aus geparsten Feldern, die über den dynamischen Pfad (gespeichert + indiziert + Term-Vektoren + `_all` + DocValues) laufen. Hebel, nach Wirkung:
  1. **Alle Parser-Felder explizit mappen** (`timestamp` fehlt z. B. komplett; FortiOS-Felder ebenfalls) mit `Store=false`, `IncludeInAll=false` ⇒ ~4–5 kB/Event **pro** Feld.
  2. `indexMapping.StoreDynamic=false` / `IndexDynamic=false` (bzw. `Dynamic=false` am DocumentMapping) — Achtung: macht nicht gemappte Felder unsuchbar.
  3. `Message` aus dem `_all` nehmen (−1,9 kB/Event), wenn Volltext feldbezogen sein darf.
- **`DelimiterSyslog`** (PRI/LF-Framing) arbeitet mit **Regex pro Byte**: ~6,3 µs/Nachricht (21 MB/s) gegen ~0,46 µs/Nachricht beim Octet-Counting (294 MB/s) — Faktor ≈ 14. Kandidat für eine Zustandsmaschine statt Regex.
- **Indexierung** dominiert die Kette (~437 µs/Event bei 2 vCPU) und bestimmt damit die Obergrenze.
