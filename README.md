_For detailed look at the goals, design, and implementation of this project, check out [these blog posts](http://www.philipotoole.com/tag/ekanite/)._

> **⚠️ WORK IN PROGRESS — nicht produktionsreif.** Dieser Fork wird aktiv refactored (siehe `docs/ROADMAP.md`). Kritische Sicherheitslücken sind offen: unauthentifizierte Query-Ports, exponiertes pprof, TCP-DoS-Vektoren und stille Datenverluste (Issues #14, #52, #78, #7, #46, #67). **Nicht für Produktionseinsatz verwenden.**

**Datenschutz:** Siehe [docs/PRIVACY.md](docs/PRIVACY.md).
# Ekanite [![Go](https://img.shields.io/badge/Go-1.24-blue?logo=go)](https://go.dev/) [![Issues](https://img.shields.io/github/issues/schankst/ekanite)](https://github.com/schankst/ekanite/issues) [![MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
*Ekanite* is a high-performance syslog server with built-in text search. Its goal is to do a couple of things, and do them well -- accept log messages over the network, and make it easy to search the messages. What it lacks in feature, it makes up for in focus. Built in [Go](http://www.golang.org), it has no external dependencies, which makes deployment easy.

Features include:

- Supports reception of log messages over UDP, TCP, and TCP with TLS.
- Full text search of all received log messages.
- Full parsing of [RFC5424](http://tools.ietf.org/html/rfc5424) headers.
- Log messages are indexed by parsed timestamp, if one is available. This means search results are presented in the order the messages occurred, not in the order they were received, ensuring sensible display even with delayed senders.
- Automatic data-retention management. Ekanite deletes indexed log data older than a configurable time period.
- Not a [JVM](https://java.com/en/download/) in sight.

Search is implemented using the [bleve](http://www.blevesearch.com/) search library. For some performance analysis of bleve, and of the sharding techniques used by Ekanite, check out [this post](http://www.philipotoole.com/increasing-bleve-performance-sharding/).

## Getting started
The quickest way to get running on OSX and Linux is to download a pre-built release binary. You can find these binaries on the [Github releases page](https://github.com/ekanite/ekanite/releases). Once installed, you can start Ekanite like so:
```bash
ekanited -datadir ~/ekanite_data # Or any directory of your choice.
```
To see all Ekanite options pass `-h` on the command line.

__If you want to build Ekanite__, either because you want the latest code or a pre-built binary for platform is not available, take a look at [CONTRIBUTING.md](https://github.com/ekanite/ekanite/blob/master/CONTRIBUTING.md).

Logs an Ekanite senden
----------------------
Ekanite nimmt Logs über TCP und/oder UDP entgegen (Flags `-tcp`, `-udp`; zum Deaktivieren des TCP-Collectors `-tcp` auf einen leeren String setzen). Das Eingabeformat wird über `-input` gewählt.

### Eingabeformate (`-input`)

| Wert | Bedeutung |
|---|---|
| `syslog` (Standard) | [RFC5424](http://tools.ietf.org/html/rfc5424)-kompatible Syslog-Zeilen (TIMESTAMP in [RFC3339](http://www.ietf.org/rfc/rfc3339.txt)). |
| `fortios` | FortiGate-FortiOS (Key-Value) |

Für `syslog` muss der Sende-Client Zeilen im folgenden Format liefern:

    <PRI>VERSION TIMESTAMP HOSTNAME APP-NAME PROC-ID MSGID MSG

Sowohl [rsyslog](http://www.rsyslog.com/) als auch [syslog-ng](http://www.balabit.com/network-security/syslog-ng) (Sende-Clients) unterstützen Templates, mit denen sich RFC5424-Zeilen einfach erzeugen und an Ekanite übertragen lassen. Beispiele:

**rsyslog**

```
# Send messages to Ekanite over TCP using the template. Assumes Ekanite is listening on 127.0.0.1:5514
$template Ekanite,"<%pri%>%protocol-version% %timestamp:::date-rfc3339% %HOSTNAME% %app-name% %procid% - %msg%\n"
*.*             @@127.0.0.1:5514;Ekanite
```
Add this template to `/etc/rsyslog.d/23-ekanite.conf` and then restart rsyslog using the command `sudo service rsyslog restart`.

**syslog-ng**

```
source s_ekanite {
	system();	# Check which OS & collect system logs
	internal();	# Collect syslog-ng logs
};
template Ekanite { template("<${PRI}>1 ${ISODATE} ${HOST} ${PROGRAM} ${PID} - $MSG\n"); template_escape(no) };
destination d_ekanite {
	tcp("127.0.0.1" port(5514) template(Ekanite));
};

log {
	source(s_ekanite);
	destination(d_ekanite);
};
```
Add this template to `/etc/syslog-ng/syslog-ng.conf` and then restart syslog-ng using the command `/etc/init.d/syslog-ng restart`.

With these changes in place rsyslog or syslog-ng will continue to send logs to any existing destination, and also forward the logs to Ekanite.

Suche
-----
Ekanite bietet zwei Suchschnittstellen: eine einfache Telnet-artige Schnittstelle (`-query`, Standard `localhost:9950`) und eine Browser-Oberfläche (`-queryhttp`, Standard `http://localhost:8080`). Beide nutzen dieselbe Query-Engine.

### Query-Syntax (bleve Query String)

Die Abfragesprache basiert auf [bleves `QueryStringQuery`](https://pkg.go.dev/github.com/blevesearch/bleve#NewQueryStringQuery). Für einfache Suchen reicht meist:

| Beispiel | Bedeutung |
|---|---|
| `login` | Term-Suche nach „login“ |
| `wp-login POST` | Mehrere Terme (kombiniert gemäß bleve-Default) |
| `login -GET` | „login“, aber Treffer mit „GET“ ausschließen (`-`-Operator) |
| `field:wert` | Feldsuche (nur für tatsächlich indizierte Felder sinnvoll; siehe Dokumentmodell) |
| `log*` / `?` | Wildcards gemäß bleve-Query-String-Syntax |

Hinweis: Die genaue Semantik (Phrasen, boolesche Operatoren, Escaping, Wildcard-Regeln) folgt bleve. Dieses README dokumentiert das beobachtbare Ekanite-Verhalten und die Grenzen; für fortgeschrittene Syntax ist die bleve-Dokumentation maßgeblich.

### Dokumentmodell und indizierte Felder

- **Gespeichert/ausgeliefert:** Als Suchtreffer liefert Ekanite die ursprüngliche Rohzeile (`Text`) der empfangenen Nachricht zurück (über Telnet zeilenweise, über HTTP als Liste).
- **Indizierter Inhalt (aktuell):** Für die Volltextsuche wird der Nachrichteninhalt als Feld `Message` indiziert (tokenisiert, für die Suche aufbereitet). Zusätzliche Zeitfelder (`ReferenceTime`, `ReceptionTime`) sind im Index-Mapping vorhanden; die aktuell indizierte Datenstruktur befüllt jedoch nur `Message`. Andere vom Eingabeparser erkannte Felder (z. B. RFC5424-Metadaten) werden nicht pauschal als eigene, dokumentierte Suchfelder versprochen.
- **Zeitbezug:** Als Referenzzeit dient — wenn parsebar — der `timestamp` aus dem geparsten Ereignis (RFC3339), sonst die Empfangszeit (`ReceptionTime`). Das beeinflusst die zeitliche Zuordnung/Indexierung; es gibt **keinen** eigenen Zeitfilter-Parameter in der Query-Schnittstelle.

### Grenzen und Ergebnisreihenfolge

- **10.000er-Limit pro Index-Suche:** Jede Suche gegen einen Zeit-Index ist auf maximal **10.000** Treffer begrenzt (`maxSearchHitSize`). Da Ekanite Daten in zeitbasierten Indizes (Shards/Indizes) hält, kann eine Abfrage mehrere Indizes durchlaufen; das Limit gilt pro Index-Suche, nicht als globaler „Top-N über alles“-Garantiewert.
- **Reihenfolge:** Treffer werden innerhalb eines Index nach Dokument-ID sortiert (zeit-/sequenzbasiert, aufsteigend) zurückgegeben. Ekanite durchsucht Indizes sequentiell von früh nach spät, sodass Ergebnisse insgesamt **zeitstabil** in aufsteigender zeitlicher Ordnung erscheinen.
- **Kein garantierter Vollständigkeitsvertrag:** Bei sehr großen Datenmengen oder Treffermengen am Limit können Ergebnisse abgeschnitten wirken. Queries sollten daher so spezifisch wie möglich formuliert werden.

### Telnet-Schnittstelle

Verbinde dich per Telnet mit dem Query-Server und sende pro Zeile eine Query. Leere Zeilen werden ignoriert. Nach den Ergebnissen sendet der Server zwei Leerzeilen als Ende-Markierung. Beispiel:

```
$ telnet 127.0.0.1 9950
Trying 127.0.0.1...
Connected to 127.0.0.1.
Escape character is '^]'.
login
<134>0 2015-05-05T23:50:17.025568+00:00 fisher apache-access - - 65.98.59.154 - - [05/May/2015:23:50:12 +0000] "GET /wp-login.php HTTP/1.0" 200 206 "-" "-"
```

Negation/Filterung (Beispiel):

```
login -GET
<134>0 2015-05-06T04:20:49.008609+00:00 fisher apache-access - - 193.104.41.186 - - [06/May/2015:04:20:46 +0000] "POST /wp-login.php HTTP/1.1" 200 206 "-" "Opera 10.00"
```

### Browser-Schnittstelle

Die Browser-Oberfläche akzeptiert dieselben bleve-Queries. Standardmäßig ist sie unter [http://localhost:8080](http://localhost:8080) erreichbar. Die Seite zeigt die Anzahl der gelieferten Treffer und die Suchdauer an.

![Data Diagram](img/eq.png)

## TLS (TCP-Eingang)

Für den TCP-Collector kann mTLS aktiviert werden, wenn **beide** Flags gesetzt sind:

- `-tlspem`: Pfad zur PEM-Datei (Zertifikat/CA-Material, wie von der aktuellen Implementierung erwartet).
- `-tlskey`: Pfad zur Schlüsseldatei.

Wichtige Einschränkungen der aktuellen Implementierung (ehrlich/bestandsnah):

- Es wird **Client-Authentifizierung erzwungen** (`RequireAndVerifyClientCert`, mTLS).
- Der private Key wird als **PKCS#1-RSA** geparst (`ParsePKCS1PrivateKey`). Andere Key-Typen (z. B. ECDSA, PKCS#8/Ed25519, je nach Format) werden beim Start mit einem TLS-Konfigurationsfehler abgelehnt.
- Es wird keine vollständige Zertifikatskette als separate Konfiguration aufgebaut; die PEM-Datei wird als einzelnes Zertifikat eingelesen und zusätzlich als Client-CA-Pool-Material verwendet.

TLS gilt nur für den TCP-Eingang. UDP bleibt unverschlüsselt.

## Diagnostics
Basic statistics and diagnostics are available (expvar/pprof). Visit `http://localhost:9951/debug/vars` to retrieve this information. The bind address can be changed via the `-diag` command-line option; to disable the diagnostics server, set `-diag` to an empty string.

## Building New Parsers
The architecture now supports the easy implementation of new parsers beyond the stock syslog in 3 easy steps:

1. In `input/parser.go` expand supportedFormats() to capture the additional _standard_ and _name_.
1. In `parser/`, create the new input format parser using appropriate regex statements.
    - Ensure that the new parser includes a `timestamp` field compatible with RFC3339, e.g. `2006-01-02T15:04:05Z07:00` 
1. Back in `input/parser.go`, update NewParser() to properly instantiate the new input format parser.

## Project Status
The project is not actively maintained, though development may re-occur in the future.

