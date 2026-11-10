package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// errUsage marks a flag parse error. The flag set has already printed the error
// and usage to stderr, so the caller should exit (code 2) without printing again.
var errUsage = errors.New("flag usage error")

// Defaults for the CLI flags.
const (
	DefaultDataDir         = "/var/opt/ekanite"
	DefaultBatchSize       = 300
	DefaultBatchTimeout    = 1000
	DefaultIndexMaxPending = 1000
	DefaultNumShards       = 4
	DefaultRetentionPeriod = "168h"
	DefaultQueryAddr       = "localhost:9950"
	DefaultHTTPQueryAddr   = "localhost:8080"
	DefaultDiagsIface      = "localhost:9951"
	DefaultTCPServer       = "localhost:5514"
	DefaultInputFormat     = "syslog"
)

// Config holds all resolved runtime configuration for ekanited (Issue #11):
// the parsed CLI flags/environment, with durations and the anonymization map
// already validated and materialized, so main() is pure orchestration.
type Config struct {
	DataDir         string
	BatchSize       int
	BatchTimeout    time.Duration
	IndexMaxPending int
	NumShards       int
	Retention       time.Duration
	IndexDuration   time.Duration
	RetentionCheck  time.Duration
	Wipe            bool
	AuditLog        string
	RawMode         string
	Anonymize       map[string]string
	AnonKey         string

	TCPIface       string
	UDPIface       string
	InputFormat    string
	Framing        string
	TLSCert        string
	TLSKey         string
	QueryIface     string
	QueryHTTPIface string
	DiagIface      string

	CPUProfile string
	MemProfile string

	ShowVersion bool
}

// parseConfig defines the flags, parses args, and returns a fully resolved
// Config. Flag/env defaults follow flag-wins-over-env: an explicit CLI flag
// overrides EKANITE_<FLAGNAME>, which overrides the compiled-in default.
func parseConfig(args []string) (*Config, error) {
	fs := flag.NewFlagSet("ekanited", flag.ContinueOnError)
	fs.Usage = printHelp

	var (
		datadir         = fs.String("datadir", envOrString("datadir", DefaultDataDir), "Set data directory")
		batchSize       = fs.Int("batchsize", envOrInt("batchsize", DefaultBatchSize), "Indexing batch size")
		batchTimeout    = fs.Int("batchtime", envOrInt("batchtime", DefaultBatchTimeout), "Indexing batch timeout, in milliseconds")
		indexMaxPending = fs.Int("maxpending", envOrInt("maxpending", DefaultIndexMaxPending), "Maximum pending index events")
		tcpIface        = fs.String("tcp", envOrString("tcp", DefaultTCPServer), "Syslog server TCP bind address in the form host:port. To disable set to empty string")
		udpIface        = fs.String("udp", envOrString("udp", ""), "Syslog server UDP bind address in the form host:port. If not set, not started")
		diagIface       = fs.String("diag", envOrString("diag", DefaultDiagsIface), "expvar and pprof bind address in the form host:port. If not set, not started")
		certPath        = fs.String("tlscert", envOrString("tlscert", ""), "path to TLS server certificate PEM file for TCP ingest (RFC 5425). If not set, TLS not activated")
		certKeyPath     = fs.String("tlskey", envOrString("tlskey", ""), "path to TLS server private key file for TCP ingest (RSA/ECDSA/Ed25519). If not set, TLS not activated")
		queryIface      = fs.String("query", envOrString("query", DefaultQueryAddr), "TCP Bind address for query server in the form host:port. To disable set to empty string")
		queryIfaceHttp  = fs.String("queryhttp", envOrString("queryhttp", DefaultHTTPQueryAddr), "TCP Bind address for http query server in the form host:port. To disable set to empty string")
		numShards       = fs.Int("numshards", envOrInt("numshards", DefaultNumShards), "Set number of shards per index")
		retentionPeriod = fs.String("retention", envOrString("retention", DefaultRetentionPeriod), "Data retention period. Minimum is 24 hours")
		indexDur        = fs.String("indexdur", envOrString("indexdur", "24h"), "Index time slice (e.g. 24h or 7d). Larger slices mean fewer, larger indexes")
		retentionCheck  = fs.String("retentioncheck", envOrString("retentioncheck", "1h"), "How often expired indexes are swept (e.g. 1h)")
		cpuProfile      = fs.String("cpuprof", envOrString("cpuprof", ""), "Where to write CPU profiling data. Not written if not set")
		memProfile      = fs.String("memprof", envOrString("memprof", ""), "Where to write memory profiling data. Not written if not set")
		inputFormat     = fs.String("input", envOrString("input", DefaultInputFormat), "Message format of input (supported: syslog, fortios, bsd)")
		framing         = fs.String("framing", envOrString("framing", "auto"), "TCP message framing: auto (default), lf, or octet (RFC 6587)")
		wipe            = fs.Bool("wipe", false, "Overwrite index files with zeroes before retention deletion (disk-write cost)")
		auditLog        = fs.String("auditlog", envOrString("auditlog", ""), "Path for retention deletion audit log (JSONL). If empty, no audit is written")
		rawMode         = fs.String("raw", envOrString("raw", "keep"), "Raw line storage: keep (default) or drop (only parsed fields, no raw line)")
		anonymize       = fs.String("anonymize", envOrString("anonymize", ""), "Field anonymization: comma-separated FIELD=ALGO, ALGO in {inet,hmac}; e.g. source_ip=inet,host=hmac. Activates data minimization (raw line not stored)")
		anonKey         = fs.String("anonkey", envOrString("anonkey", ""), "HMAC key for anonymize=hmac (from env EKANITE_ANONKEY; never logged)")
		showVersion     = fs.Bool("version", false, "Print version information and exit")
	)

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil, flag.ErrHelp
		}
		return nil, fmt.Errorf("%w: %v", errUsage, err)
	}

	cfg := &Config{
		DataDir:         *datadir,
		BatchSize:       *batchSize,
		BatchTimeout:    time.Duration(*batchTimeout) * time.Millisecond,
		IndexMaxPending: *indexMaxPending,
		NumShards:       *numShards,
		Wipe:            *wipe,
		AuditLog:        *auditLog,
		RawMode:         *rawMode,
		AnonKey:         *anonKey,
		TCPIface:        *tcpIface,
		UDPIface:        *udpIface,
		InputFormat:     *inputFormat,
		Framing:         *framing,
		TLSCert:         *certPath,
		TLSKey:          *certKeyPath,
		QueryIface:      *queryIface,
		QueryHTTPIface:  *queryIfaceHttp,
		DiagIface:       *diagIface,
		CPUProfile:      *cpuProfile,
		MemProfile:      *memProfile,
		ShowVersion:     *showVersion,
	}

	var err error
	if cfg.Retention, err = parseDurationFlag(*retentionPeriod, "retention"); err != nil {
		return nil, fmt.Errorf("failed to parse retention period '%s': %w", *retentionPeriod, err)
	}
	if cfg.IndexDuration, err = parseDurationFlag(*indexDur, "indexdur"); err != nil {
		return nil, fmt.Errorf("failed to parse index duration '%s': %w", *indexDur, err)
	}
	if cfg.RetentionCheck, err = parseDurationFlag(*retentionCheck, "retentioncheck"); err != nil {
		return nil, fmt.Errorf("failed to parse retention check interval '%s': %w", *retentionCheck, err)
	}
	if cfg.Anonymize, err = parseAnonymize(*anonymize); err != nil {
		return nil, err
	}

	return cfg, nil
}

// parseAnonymize parses the -anonymize spec ("FIELD=ALGO,FIELD=ALGO") into a
// map. ALGO must be "inet" or "hmac" (DSGVO #93/#95).
func parseAnonymize(spec string) (map[string]string, error) {
	anon := map[string]string{}
	if spec == "" {
		return anon, nil
	}
	for _, part := range strings.Split(spec, ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			return nil, fmt.Errorf("invalid -anonymize entry %q (want FIELD=ALGO)", part)
		}
		algo := strings.ToLower(strings.TrimSpace(kv[1]))
		if algo != "inet" && algo != "hmac" {
			return nil, fmt.Errorf("invalid -anonymize algorithm %q (want inet or hmac)", kv[1])
		}
		anon[strings.TrimSpace(kv[0])] = algo
	}
	return anon, nil
}

// envKey returns the environment variable name for a CLI flag: EKANITE_<FLAGNAME>.
func envKey(flagName string) string {
	return "EKANITE_" + strings.ToUpper(flagName)
}

// envOrString returns the value of EKANITE_<flagName> if set (including empty),
// otherwise fallback. Using LookupEnv preserves intentionally empty values, which
// matters for flags where an empty string disables a component (e.g. -tcp, -udp).
func envOrString(flagName, fallback string) string {
	if v, ok := os.LookupEnv(envKey(flagName)); ok {
		return v
	}
	return fallback
}

// envOrInt returns the parsed integer value of EKANITE_<flagName> if set and
// valid, otherwise fallback. Invalid env values are ignored so a bad environment
// does not prevent startup; the explicit CLI flag still wins when provided.
func envOrInt(flagName string, fallback int) int {
	v, ok := os.LookupEnv(envKey(flagName))
	if !ok {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

// parseDurationFlag parses a duration flag, additionally accepting a trailing
// "d" suffix as days (e.g. "7d") beyond what time.ParseDuration understands.
func parseDurationFlag(s, name string) (time.Duration, error) {
	if strings.HasSuffix(s, "d") {
		days, err := strconv.ParseFloat(strings.TrimSuffix(s, "d"), 64)
		if err != nil {
			return 0, err
		}
		return time.Duration(days * float64(24*time.Hour)), nil
	}
	return time.ParseDuration(s)
}
