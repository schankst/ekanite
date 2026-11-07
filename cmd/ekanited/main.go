package main

import (
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ekanite/ekanite"
	"github.com/ekanite/ekanite/input"
	"github.com/ekanite/ekanite/status"
)

var (
	version = "dev"
	commit  = "unknown"
)

// Flag set
var fs *flag.FlagSet

// Types
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

// SIGHUP-Reload (out of scope for Issue #65): uncritical runtime values such as
// retention period and log level could become reloadable on SIGHUP in a follow-up
// issue. Structural/startup flags (bind addresses, datadir, shard count, batch
// sizing, TLS paths) are not reloadable without restart.

func versionString() string {
	return fmt.Sprintf("ekanite %s (%s) %s", version, commit, runtime.Version())
}

func main() {
	fs = flag.NewFlagSet("", flag.ExitOnError)
	var (
		// Flag defaults are read from EKANITE_<FLAGNAME> (flag name uppercased) when
		// set; otherwise the compiled-in default is used. An explicitly provided CLI
		// flag always wins over the environment-derived default (flag-wins-over-env).
		datadir         = fs.String("datadir", envOrString("datadir", DefaultDataDir), "Set data directory")
		batchSize       = fs.Int("batchsize", envOrInt("batchsize", DefaultBatchSize), "Indexing batch size")
		batchTimeout    = fs.Int("batchtime", envOrInt("batchtime", DefaultBatchTimeout), "Indexing batch timeout, in milliseconds")
		indexMaxPending = fs.Int("maxpending", envOrInt("maxpending", DefaultIndexMaxPending), "Maximum pending index events")
		tcpIface        = fs.String("tcp", envOrString("tcp", DefaultTCPServer), "Syslog server TCP bind address in the form host:port. To disable set to empty string")
		udpIface        = fs.String("udp", envOrString("udp", ""), "Syslog server UDP bind address in the form host:port. If not set, not started")
		diagIface       = fs.String("diag", envOrString("diag", DefaultDiagsIface), "expvar and pprof bind address in the form host:port. If not set, not started")
		caPemPath       = fs.String("tlspem", envOrString("tlspem", ""), "path to CA PEM file for TLS-enabled TCP server. If not set, TLS not activated")
		caKeyPath       = fs.String("tlskey", envOrString("tlskey", ""), "path to CA key file for TLS-enabled TCP server. If not set, TLS not activated")
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
	fs.Usage = printHelp
	_ = fs.Parse(os.Args[1:]) // ExitOnError: parse errors terminate via os.Exit already

	// Restrict file creation to the owner only (Issue #90): datadir, index
	// shards and audit log must not be world-readable. Combined with explicit
	// 0700/0600 modes this enforces owner-only access to stored logs.
	syscall.Umask(0077)

	if *showVersion {
		fmt.Println(versionString())
		os.Exit(0)
	}

	// Data minimization & anonymization policy (DSGVO #93/#95). Parse
	// -anonymize FIELD=ALGO,... into the transform config before the engine
	// starts indexing.
	anon := map[string]string{}
	if *anonymize != "" {
		for _, part := range strings.Split(*anonymize, ",") {
			kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
			if len(kv) != 2 {
				log.Fatalf("invalid -anonymize entry %q (want FIELD=ALGO)", part)
			}
			algo := strings.ToLower(strings.TrimSpace(kv[1]))
			if algo != "inet" && algo != "hmac" {
				log.Fatalf("invalid -anonymize algorithm %q (want inet or hmac)", kv[1])
			}
			anon[strings.TrimSpace(kv[0])] = algo
		}
	}
	ekanite.SetTransform(ekanite.TransformConfig{
		RawMode:   *rawMode,
		Anonymize: anon,
		AnonKey:   []byte(*anonKey),
	})
	if *rawMode == "drop" || len(anon) > 0 {
		log.Printf("data minimization active: raw=%s, anonymize=%d field(s) (raw line not stored)",
			*rawMode, len(anon))
	}

	absDataDir, err := filepath.Abs(*datadir)
	if err != nil {
		log.Fatalf("failed to get absolute data path for '%s': %s", *datadir, err.Error())
	}

	// Get the retention period.
	retention, err := parseDurationFlag(*retentionPeriod, "retention")
	if err != nil {
		log.Fatalf("failed to parse retention period '%s'", *retentionPeriod)
	}
	indexDuration, err := parseDurationFlag(*indexDur, "indexdur")
	if err != nil {
		log.Fatalf("failed to parse index duration '%s'", *indexDur)
	}
	retentionCheckIntv, err := parseDurationFlag(*retentionCheck, "retentioncheck")
	if err != nil {
		log.Fatalf("failed to parse retention check interval '%s'", *retentionCheck)
	}

	log.SetFlags(log.LstdFlags)
	log.SetPrefix("[ekanite] ")
	log.Printf("ekanite started using %s for index storage", absDataDir)

	runtime.GOMAXPROCS(runtime.NumCPU())
	log.Println("GOMAXPROCS set to", runtime.GOMAXPROCS(0))

	// Start the expvar handler if requested.
	if *diagIface != "" {
		startDiagServer(*diagIface)
	}

	// Create and open the Engine.
	engine := ekanite.NewEngine(absDataDir)
	engine.NumShards = *numShards
	engine.RetentionPeriod = retention
	engine.IndexDuration = indexDuration
	engine.RetentionCheckInterval = retentionCheckIntv
	engine.WipeEnabled = *wipe
	if *auditLog != "" {
		f, err := os.OpenFile(*auditLog, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			log.Fatalf("failed to open audit log '%s': %s", *auditLog, err.Error())
		}
		engine.AuditLogger = log.New(f, "", 0)
	}

	if err := engine.Open(); err != nil {
		log.Fatalf("failed to open engine: %s", err.Error())
	}
	log.Printf("engine opened with shard number of %d, retention period of %s, index duration of %s",
		engine.NumShards, engine.RetentionPeriod, engine.IndexDuration)

	// Start the simple query server if requested.
	if *queryIface != "" {
		startQueryServer(*queryIface, engine)
	}

	// Start the http query server if requested.
	if *queryIfaceHttp != "" {
		startHTTPQueryServer(*queryIfaceHttp, engine)
	}

	// Create and start the batcher.
	batcherTimeout := time.Duration(*batchTimeout) * time.Millisecond
	batcher := ekanite.NewBatcher(engine, *batchSize, batcherTimeout, *indexMaxPending)

	errChan := make(chan error)
	if err := batcher.Start(errChan); err != nil {
		log.Fatalf("failed to start indexing batcher: %s", err.Error())
	}
	log.Printf("batching configured with size %d, timeout %s, max pending %d",
		*batchSize, batcherTimeout, *indexMaxPending)

	// Start draining batcher errors.
	go drainLog("error indexing batch", errChan)

	// Start TCP collector if requested.
	if *tcpIface != "" {
		var tlsConfig *tls.Config
		if *caPemPath != "" && *caKeyPath != "" {
			tlsConfig, err = newTLSConfig(*caPemPath, *caKeyPath)
			if err != nil {
				log.Fatalf("failed to configure TLS: %s", err.Error())
			}
			log.Printf("TLS successfully configured")
		}

		if err := startTCPCollector(*tcpIface, *inputFormat, *framing, tlsConfig, batcher); err != nil {
			log.Fatalf("failed to start TCP collector: %s", err.Error())
		}
		log.Printf("TCP collector listening to %s", *tcpIface)
	}

	// Start UDP collector if requested.
	if *udpIface != "" {
		if err := startUDPCollector(*udpIface, *inputFormat, batcher); err != nil {
			log.Fatalf("failed to start UDP collector: %s", err.Error())
		}
		log.Printf("UDP collector listening to %s", *udpIface)
	}

	// Start profiling.
	startProfile(*cpuProfile, *memProfile)

	// Wait forever for signals.
	waitForSignals()

	engine.Close()

	stopProfile()
}

func startTCPCollector(iface, format, framing string, tls *tls.Config, batcher *ekanite.Batcher) error {
	collector, err := input.NewCollector("tcp", iface, format, framing, tls)
	if err != nil {
		return fmt.Errorf("failed to create TCP collector: %w", err)
	}
	if err := collector.Start(batcher.C()); err != nil {
		return fmt.Errorf("failed to start TCP collector: %w", err)
	}

	return nil
}

func startUDPCollector(iface, format string, batcher *ekanite.Batcher) error {
	collector, err := input.NewCollector("udp", iface, format, "", nil)
	if err != nil {
		return fmt.Errorf("failed to create UDP collector: %w", err)
	}
	if err := collector.Start(batcher.C()); err != nil {
		return fmt.Errorf("failed to start UDP collector: %w", err)
	}

	return nil
}

func startQueryServer(iface string, engine *ekanite.Engine) {
	server := ekanite.NewServer(iface, engine)
	if server == nil {
		log.Fatal("failed to create query server")
	}
	if err := server.Start(); err != nil {
		log.Fatalf("failed to start query server: %s", err.Error())
	}
	log.Printf("query server listening on %s", iface)
}

func startHTTPQueryServer(iface string, engine *ekanite.Engine) {
	server := ekanite.NewHTTPServer(iface, engine)
	if server == nil {
		log.Fatal("failed to create HTTP query server")
	}
	if err := server.Start(); err != nil {
		log.Fatalf("failed to start HTTP query server: %s", err.Error())
	}
	log.Printf("HTTP query server listening on %s", iface)
}

func startDiagServer(iface string) {
	diagServer := status.NewService(iface)
	diagServer.BuildInfo = map[string]interface{}{
		"version":    version,
		"commit":     commit,
		"go_version": runtime.Version(),
	}
	if err := diagServer.Start(); err != nil {
		log.Fatalf("failed to start status server on %s: %s", iface, err.Error())
	}
	log.Printf("diagnostic server listening on %s", iface)
}

func newTLSConfig(caPemPath, caKeyPath string) (*tls.Config, error) {
	var config *tls.Config

	caPem, err := ioutil.ReadFile(caPemPath)
	if err != nil {
		return nil, err
	}
	ca, err := x509.ParseCertificate(caPem)
	if err != nil {
		return nil, err
	}

	caKey, err := ioutil.ReadFile(caKeyPath)
	if err != nil {
		return nil, err
	}
	key, err := x509.ParsePKCS1PrivateKey(caKey)
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	pool.AddCert(ca)

	cert := tls.Certificate{
		Certificate: [][]byte{caPem},
		PrivateKey:  key,
	}

	config = &tls.Config{
		ClientAuth:   tls.RequireAndVerifyClientCert,
		Certificates: []tls.Certificate{cert},
		ClientCAs:    pool,
	}

	config.Rand = rand.Reader

	return config, nil
}

// drainLog drains errors from the channel and simply logs them
func drainLog(msg string, errChan <-chan error) {
	for err := range errChan {
		if err != nil {
			log.Printf("%s: %s", msg, err.Error())
		}
	}
}

// waitForSignals blocks until a signal is received.
func waitForSignals() {
	// Set up signal handling.
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)

	// Block until one of the signals above is received
	<-signalCh
	log.Println("signal received, shutting down...")
}

// prof stores the file locations of active profiles.
var prof struct {
	cpu *os.File
	mem *os.File
}

// StartProfile initializes the cpu and memory profile, if specified.
func startProfile(cpuprofile, memprofile string) {
	if cpuprofile != "" {
		f, err := os.Create(cpuprofile)
		if err != nil {
			log.Fatalf("cpuprofile: %v", err)
		}
		log.Printf("writing CPU profile to: %s\n", cpuprofile)
		prof.cpu = f
		if err := pprof.StartCPUProfile(prof.cpu); err != nil {
			log.Fatalf("failed to start CPU profile: %v", err)
		}
	}

	if memprofile != "" {
		f, err := os.Create(memprofile)
		if err != nil {
			log.Fatalf("memprofile: %v", err)
		}
		log.Printf("writing memory profile to: %s\n", memprofile)
		prof.mem = f
		runtime.MemProfileRate = 4096
	}

}

// StopProfile closes the cpu and memory profiles if they are running.
func stopProfile() {
	if prof.cpu != nil {
		pprof.StopCPUProfile()
		prof.cpu.Close()
		log.Println("CPU profile stopped")
	}
	if prof.mem != nil {
		if err := pprof.Lookup("heap").WriteTo(prof.mem, 0); err != nil {
			log.Printf("failed to write memory profile: %v", err)
		}
		prof.mem.Close()
		log.Println("memory profile stopped")
	}
}

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

TLS:
  -tlspem string        path to CA PEM file for TLS-enabled TCP server. If not set, TLS not activated
  -tlskey string        path to CA key file for TLS-enabled TCP server. If not set, TLS not activated

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
