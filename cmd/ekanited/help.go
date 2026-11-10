package main

import "fmt"

// printHelp writes the usage/help text. It is also wired as the flag set's
// Usage func, so it is shown for -h and on flag parse errors.
func printHelp() {
	fmt.Print(`ekanited [options]

Data/Engine:
  -datadir string       Set data directory (default "/var/opt/ekanite")
  -indexdur string      Index time slice (e.g. 24h or 7d). Larger slices mean fewer, larger indexes (default "24h")
  -retentioncheck string  How often expired indexes are swept (e.g. 1h) (default "1h")
  -wipe                 Overwrite index files with zeroes before retention deletion (disk-write cost)
  -auditlog string      Path for retention deletion audit log (JSONL). If empty, no audit is written
  -raw string           Raw line storage: keep (default) or drop (only parsed fields, no raw line)
  -anonymize string     Field anonymization: FIELD=ALGO,... ALGO in {inet,hmac} (activates data minimization)
  -anonkey string       HMAC key for anonymize=hmac (from env EKANITE_ANONKEY; never logged)
  -batchsize int        Indexing batch size (default 300)
  -batchtime int        Indexing batch timeout, in milliseconds (default 1000)
  -maxpending int       Maximum pending index events (default 1000)
  -numshards int        Set number of shards per index (default 4)
  -retention string     Data retention period. Minimum is 24 hours (default "168h")

Ingest:
  -tcp string           Syslog server TCP bind address in the form host:port. To disable set to empty string (default "localhost:5514")
  -udp string           Syslog server UDP bind address in the form host:port. If not set, not started
  -input string         Message format of input (supported: syslog, fortios, bsd) (default "syslog")
  -framing string       TCP message framing: auto (default), lf, or octet (RFC 6587)

Query:
  -query string         TCP Bind address for query server in the form host:port. To disable set to empty string (default "localhost:9950")
  -queryhttp string     TCP Bind address for http query server in the form host:port. To disable set to empty string (default "localhost:8080")

Diagnostics:
  -diag string          expvar and pprof bind address in the form host:port. If not set, not started (default "localhost:9951")

Profiling:
  -cpuprof string       Where to write CPU profiling data. Not written if not set
  -memprof string       Where to write memory profiling data. Not written if not set

TLS (TCP ingest, RFC 5425):
  -tlscert string       path to TLS server certificate PEM file (chain supported). If not set, TLS not activated
  -tlskey string        path to TLS server private key file (RSA/ECDSA/Ed25519). If not set, TLS not activated
  SIGHUP reloads the certificates without restart.

General:
  -version              Print version information and exit
  -h, -help             Print this help

Environment variables:
  Most flags can be set via EKANITE_<FLAGNAME> (flag name uppercased), e.g.
  EKANITE_BATCHSIZE, EKANITE_TCP, EKANITE_RETENTION, EKANITE_DATADIR. An
  explicit CLI flag overrides the environment variable. -version has no env var.

Supported input formats:
  syslog                RFC5424-compatible syslog lines
  fortios               FortiGate FortiOS key-value lines
  bsd                   RFC 3164 (BSD) syslog lines

Build with version info:
  go build -ldflags "-X main.version=1.4.0 -X main.commit=$(git rev-parse --short HEAD)" ./cmd/ekanited
`)
}
