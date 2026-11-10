# Kapazität & Dimensionierung (Issue #77)

**Zweck:** reproduzierbare Richtwerte für Betrieb und Dimensionierung („welche Last schafft X RAM/CPU?“) und eine Grundlage für Performance-Vergleiche (u. a. die Engine-Migration, #118).

## Benchmarks (Quelle der Zahlen)

| Datei | Benchmarks |
|---|---|
| `input/bench_test.go` | `BenchmarkDelimiterOctet`, `BenchmarkDelimiterSyslog`, `BenchmarkParserRFC5424`, `BenchmarkParserBSD`, `BenchmarkParserFortiOS`, `BenchmarkIngestPath` |
| `bench_test.go` | `BenchmarkBatchIndexing` (100k Events, Produktions-Batchgröße/-Shards) |

```sh
# lokal (stabile Mikro-/Parser-Werte):
go test -run '^$' -bench . -benchmem ./...

# 100k-Indexierung (gebunden, ein Messpunkt):
go test -run '^$' -bench BenchmarkBatchIndexing -benchmem -benchtime=1x ./
```

In der CI läuft der Job **`Benchmark (informativ)`** (nicht blockierend, `-benchtime=1x`, Artefakt `benchmarks`).

## Referenzmessung

Umgebung: linux/amd64, **2 vCPU** (Intel Xeon Skylake), Go 1.26; Engine mit `-numshards 4`, `-batchsize 300` (Produktions-Defaults), Zeilenlänge ~100 B (RFC 5424 ohne strukturierte Daten).

| Benchmark | ns/op | Durchsatz | B/op | allocs/op |
|---|---:|---:|---:|---:|
| `DelimiterOctet` (100 Frames) | 46.257 | 294 MB/s | 14.904 | 106 |
| `DelimiterSyslog` (100 Nachrichten) | 634.699 | 21 MB/s | 81.767 | 203 |
| `ParserRFC5424` (1 Zeile) | 7.086 | 18,6 MB/s | 2.542 | 30 |
| `ParserBSD` (1 Zeile) | 1.759 | 25,0 MB/s | 882 | 11 |
| `ParserFortiOS` (1 Zeile) | 3.967 | 74,4 MB/s | 4.528 | 42 |
| `IngestPath` (Parse+Event) | 7.304 | — | 2.784 | 32 |
| `BatchIndexing` (100k Events) | ≈1,40 · 10¹⁰ (14,0 s) | **≈7.140 Events/s** | 9,87 GB | 94,9 Mio |

Pro Nachricht: Octet-Framing ≈ 0,46 µs · PRI/LF-Framing ≈ 6,3 µs · RFC5424-Parse ≈ 7,1 µs · **Indexierung ≈ 140 µs**.

## Kapazitätstabelle (Richtwerte)

Indexierung skaliert mit Kernen (Shards werden parallel indiziert), bis Disk-I/O begrenzt:

| CPU-Kerne | Events/s (Indexierung) | Anmerkung |
|---:|---:|---|
| 2 | **~7.100** | gemessen (Referenz) |
| 4 | ~14.000 | Annahme: lineare Skalierung |
| 8 | ~28.000 | Annahme; ab hier Disk-I/O prüfen |

## Speicher

- **Disk:** **~10,5 KiB/Event** (gemessen: 1,08 GB für 100k Events) → 1 Mio Events ≈ 10,5 GB; 24 h bei 100 Events/s (~8,6 Mio) ≈ 90 GB. Das Mapping speichert `Message` (Rohzeile) **und** die #34-Felder; die Rohzeile ist der größte Anteil (Datenminimierung `-rawmode drop` reduziert sie maßgeblich, vgl. #93/#95).
- **RAM:** das Allokationsvolumen der Indexierung beträgt ~96 KiB/Event (kumulativ, GC-bereinigt — **nicht** Peak-RSS). Die Puffergröße begrenzen `-batchsize` (Default 300) und `-maxpending` (Default 1000); der Peak-RSS wurde noch nicht gemessen (offen).

## Annahmen

- Zeilenlänge ~100 B, RFC 5424 ohne strukturierte Daten; bleve v1.0.14 (upsidedown) mit Default-Mapping (#34); lokale SSD; kein TLS; keine Vektor-/Geo-Felder.
- Änderungen an Retention, Shard-Zahl, Batchgröße oder Mapping verschieben die Werte.

## Regressionsregel

> Ein Benchmark-Ergebnis, das **> 10 %** schlechter ist als die Referenz, ist ein Review-Blocker (Issue #39, #77).

Der CI-Job ist **informativ** (ein Messpunkt, geteilte Runner = verrauscht). Der belastbare Vergleich erfolgt lokal mit `benchstat` auf derselben Maschine:

```sh
go test -run '^$' -bench . -count=10 ./... > new.txt   # Baseline: old.txt
benchstat old.txt new.txt
```

## Bekannte Engpässe

- **`DelimiterSyslog`** (PRI/LF-Framing) arbeitet mit **Regex pro Byte**: ~6,3 µs/Nachricht (21 MB/s) gegen ~0,46 µs/Nachricht beim Octet-Counting (294 MB/s) — Faktor ≈ 14. Kandidat für eine Zustandsmaschine statt Regex.
- **Indexierung** dominiert die Kette (~140 µs/Event bei 2 vCPU) und bestimmt damit die Obergrenze.
