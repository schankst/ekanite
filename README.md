# Ekanite [![Go](https://img.shields.io/badge/Go-1.26-blue?logo=go)](https://go.dev/) [![Issues](https://img.shields.io/github/issues/schankst/ekanite)](https://github.com/schankst/ekanite/issues) [![MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

> **⚠️ WORK IN PROGRESS — nicht produktionsreif.** Dieser Fork wird aktiv refactored (Stand/Plan: [`docs/LAGEBILD.md`](docs/LAGEBILD.md), [`docs/ROADMAP.md`](docs/ROADMAP.md)). Kritische Sicherheitslücken sind offen: unauthentifizierte Query-Ports, exponiertes pprof, TCP-DoS-Vektoren, stille Datenverluste (Issues #14, #52, #78, #67). **Nicht für den Produktiveinsatz verwenden.**

*Ekanite* ist ein hochperformanter Syslog-Server mit eingebauter Textsuche: Log-Nachrichten über das Netzwerk entgegennehmen und durchsuchbar machen — fokussiert, in Go, ohne externe Laufzeitabhängigkeiten.

> **Diese README ist während des Refactorings bewusst knapp gehalten.** Detaildokumentation liegt unter [`docs/`](#dokumentation). Alle CLI-Optionen zeigt `ekanited -h`.

## Funktionen

- Empfang über UDP, TCP und TCP+TLS (RFC 5425).
- Volltextsuche über alle empfangenen Nachrichten (via [bleve](https://www.blevesearch.com/)).
- Parsing der RFC-5424-Header (Structured Data, NILVALUE, BOM) sowie RFC 3164 (BSD) und FortiOS.
- Indexierung nach geparstem Zeitstempel → Ergebnisse erscheinen in zeitlicher Reihenfolge, auch bei verspäteten Sendern.
- Automatische Daten-Retention (konfigurierbar).

## Schnellstart

```bash
go build ./...                       # Go 1.26+
./ekanited -datadir ~/ekanite_data   # oder ein anderes Verzeichnis
ekanited -h                          # alle Optionen
```

Versionierte Builds betten Version/Commit via `-ldflags` ein (siehe [CONTRIBUTING.md](CONTRIBUTING.md) und [docs/RELEASING.md](docs/RELEASING.md)).

## Logs senden

Ekanite nimmt Logs über `-tcp` und/oder `-udp` entgegen (Bind-Adresse leer = Collector aus). Das Eingabeformat wählt `-input`: `syslog` (Standard), `fortios`, `bsd`. Für `syslog` gilt:

```
<PRI>VERSION TIMESTAMP HOSTNAME APP-NAME PROC-ID MSGID MSG
```

Sende-Clients wie [rsyslog](http://www.rsyslog.com/) und [syslog-ng](http://www.balabit.com/network-security/syslog-ng) erzeugen RFC-5424-Zeilen über Templates:

**rsyslog** — in `/etc/rsyslog.d/23-ekanite.conf`, danach Dienst neu starten:

```
$template Ekanite,"<%pri%>%protocol-version% %timestamp:::date-rfc3339% %HOSTNAME% %app-name% %procid% - %msg%\n"
*.*             @@127.0.0.1:5514;Ekanite
```

**syslog-ng** — in `/etc/syslog-ng/syslog-ng.conf`, danach Dienst neu starten:

```
template Ekanite { template("<${PRI}>1 ${ISODATE} ${HOST} ${PROGRAM} ${PID} - $MSG\n"); template_escape(no) };
destination d_ekanite { tcp("127.0.0.1" port(5514) template(Ekanite)); };
log { source(s_sys); destination(d_ekanite); };
```

Bestehende Ziele bleiben dabei unberührt; die Logs werden zusätzlich an Ekanite weitergeleitet.

## Suchen

Zwei Schnittstellen, eine Engine: Telnet-artig (`-query`, Standard `localhost:9950`) und Browser (`-queryhttp`, Standard `http://localhost:8080`).

Die Abfragesprache ist [bleves `QueryStringQuery`](https://pkg.go.dev/github.com/blevesearch/bleve#NewQueryStringQuery): Terme (`login`), Ausschluss (`login -GET`), Feldsuche (`field:wert`), Wildcards (`log*`). Der Nachrichteninhalt wird als Feld `Message` indexiert; geparste Felder (`app`, `host`, `severity`, `source_ip`, …) sind als Suchfelder verfügbar.

Grenzen: max. **10.000** Treffer je Index-Suche; Ergebnisse kommen zeitstabil aufsteigend (früh → spät).

## Betrieb

- **TLS (TCP-Ingest, RFC 5425):** `-tlscert` + `-tlskey` (RSA/ECDSA/Ed25519); ohne Client-Zertifikate. `SIGHUP` lädt die Zertifikate neu (Rotation ohne Neustart).
- **Diagnostics:** expvar/pprof über `-diag` (Standard `localhost:9951`) — nur intern erreichbar machen.
- **Shutdown:** `SIGINT`/`SIGTERM` stoppen geordnet (Listener → Batcher-Flush → Engine).
- **Deployment:** [`deploy/systemd/ekanited.service`](deploy/systemd/ekanited.service), [`Dockerfile`](Dockerfile); Datadir (`-datadir`) muss persistent sein.

## Konfiguration

Jede Option ist CLI-Flag oder Umgebungsvariable `EKANITE_<FLAGNAME>` (Flag-Name groß); ein explizites Flag überschreibt die Env-Var, diese den kompilierten Default. Maßgeblich ist `ekanited -h`.

| Flag / Env | Zweck |
|---|---|
| `-datadir` / `EKANITE_DATADIR` | Speicherort der Indexdaten (persistent) |
| `-tcp` / `EKANITE_TCP` | TCP-Ingest-Bind (`host:port`; leer = aus) |
| `-udp` / `EKANITE_UDP` | UDP-Ingest-Bind (leer = aus; unverschlüsselt) |
| `-input` / `EKANITE_INPUT` | Eingabeformat: `syslog`, `fortios`, `bsd` |
| `-framing` / `EKANITE_FRAMING` | TCP-Framing: `auto` (Standard), `lf`, `octet` |
| `-tlscert` / `EKANITE_TLSCERT` | TLS-Server-Zertifikat (mit `-tlskey`) |
| `-tlskey` / `EKANITE_TLSKEY` | TLS-Server-Private-Key (RSA/ECDSA/Ed25519) |
| `-query` / `EKANITE_QUERY` | Telnet-Query-Bind (leer = aus) |
| `-queryhttp` / `EKANITE_QUERYHTTP` | HTTP-Query-Bind (leer = aus) |
| `-diag` / `EKANITE_DIAG` | expvar/pprof-Bind (leer = aus) |
| `-retention` / `EKANITE_RETENTION` | Aufbewahrungsdauer (min. 24h; Default `168h`) |
| `-retentioncheck` / `EKANITE_RETENTIONCHECK` | Intervall der Retention-Prüfung |
| `-wipe`, `-auditlog` | Null-Überschreibung + Lösch-Auditlog (siehe [docs/PRIVACY.md](docs/PRIVACY.md)) |
| `-indexdur` / `EKANITE_INDEXDUR` | Zeit-Slice pro Index (Default `24h`) |
| `-numshards` / `EKANITE_NUMSHARDS` | Shards je Index (Default 4) |
| `-batchsize` / `EKANITE_BATCHSIZE` | Dokumente je Index-Batch |
| `-batchtime` / `EKANITE_BATCHTIME` | Batch-Flush-Timeout (ms) |
| `-maxpending` / `EKANITE_MAXPENDING` | Puffer ausstehender Index-Events |
| `-raw`, `-anonymize`, `-anonkey` | Datensparsamkeit/Anonymisierung (siehe [docs/PRIVACY.md](docs/PRIVACY.md)) |
| `-cpuprof`, `-memprof` | CPU-/Heap-Profil (Diagnose, ad hoc) |
| `-version`, `-h` | Version ausgeben / Hilfe |

## Dokumentation

| Dokument | Inhalt |
|---|---|
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | Komponenten, Datenfluss, Parser/Delimiter |
| [docs/OPERATIONS.md](docs/OPERATIONS.md) | Backup/Restore-Runbook |
| [docs/PRIVACY.md](docs/PRIVACY.md) | Datenschutz/DSGVO (Retention, Wipe, Anonymisierung) |
| [docs/RELEASING.md](docs/RELEASING.md) | Release-Pipeline (GoReleaser, GHCR, SBOM) |
| [docs/QUALITY-GATES.md](docs/QUALITY-GATES.md) | CI-Gates-Vertrag |
| [docs/ROADMAP.md](docs/ROADMAP.md) · [docs/LAGEBILD.md](docs/LAGEBILD.md) | Planung · aktueller Stand |
| [CONTRIBUTING.md](CONTRIBUTING.md) | Beitrags- und Build-Hinweise |

## Neue Parser

Neue Eingabeformate in drei Schritten ergänzen:

1. In `input/parser.go` `supportedFormats()` um Standard + Name erweitern.
2. In `parser/` den neuen Parser (Regex) anlegen; das Ergebnis muss ein RFC3339-kompatibles `timestamp`-Feld enthalten.
3. In `input/parser.go` `NewParser()` um die Instanziierung des neuen Formats erweitern.

## Lizenz

MIT — siehe [LICENSE](LICENSE).
