package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"github.com/schankst/ekanite"
)

var (
	version = "dev"
	commit  = "unknown"
)

func versionString() string {
	return fmt.Sprintf("ekanite %s (%s) %s", version, commit, runtime.Version())
}

// main is pure orchestration (Issue #11): parse config, then build and run the
// components, then shut down in order.
func main() {
	cfg, err := parseConfig(os.Args[1:])
	if err != nil {
		switch {
		case errors.Is(err, flag.ErrHelp):
			os.Exit(0) // usage already printed
		case errors.Is(err, errUsage):
			os.Exit(2) // flag set already printed the error and usage
		default:
			log.Fatal(err)
		}
	}

	// Restrict file creation to the owner only (Issue #90): datadir, index
	// shards and audit log must not be world-readable. Combined with explicit
	// 0700/0600 modes this enforces owner-only access to stored logs. Windows
	// has no process umask; a no-op there (guarded by build tag, #138).
	restrictFileCreation()

	if cfg.ShowVersion {
		fmt.Println(versionString())
		os.Exit(0)
	}

	// Data minimization & anonymization policy (DSGVO #93/#95). The transform
	// config is applied before the engine starts indexing.
	ekanite.SetTransform(ekanite.TransformConfig{
		RawMode:   cfg.RawMode,
		Anonymize: cfg.Anonymize,
		AnonKey:   []byte(cfg.AnonKey),
	})
	if cfg.RawMode == "drop" || len(cfg.Anonymize) > 0 {
		log.Printf("data minimization active: raw=%s, anonymize=%d field(s) (raw line not stored)",
			cfg.RawMode, len(cfg.Anonymize))
	}

	absDataDir, err := filepath.Abs(cfg.DataDir)
	if err != nil {
		log.Fatalf("failed to get absolute data path for '%s': %s", cfg.DataDir, err.Error())
	}

	log.SetFlags(log.LstdFlags)
	log.SetPrefix("[ekanite] ")
	log.Printf("ekanite started using %s for index storage", absDataDir)

	runtime.GOMAXPROCS(runtime.NumCPU())
	log.Println("GOMAXPROCS set to", runtime.GOMAXPROCS(0))

	// Start the expvar handler if requested.
	var components closers
	if cfg.DiagIface != "" {
		components.diag = startDiagServer(cfg.DiagIface)
	}

	// Create and open the Engine.
	engine := ekanite.NewEngine(absDataDir)
	engine.NumShards = cfg.NumShards
	engine.RetentionPeriod = cfg.Retention
	engine.IndexDuration = cfg.IndexDuration
	engine.RetentionCheckInterval = cfg.RetentionCheck
	engine.WipeEnabled = cfg.Wipe
	if cfg.AuditLog != "" {
		f, err := os.OpenFile(cfg.AuditLog, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			log.Fatalf("failed to open audit log '%s': %s", cfg.AuditLog, err.Error())
		}
		engine.AuditLogger = log.New(f, "", 0)
	}

	if err := engine.Open(); err != nil {
		log.Fatalf("failed to open engine: %s", err.Error())
	}
	log.Printf("engine opened with shard number of %d, retention period of %s, index duration of %s",
		engine.NumShards, engine.RetentionPeriod, engine.IndexDuration)

	// Start the query servers if requested.
	startQueryServers(cfg, engine, &components)

	// Create and start the batcher.
	batcher := ekanite.NewBatcher(engine, cfg.BatchSize, cfg.BatchTimeout, cfg.IndexMaxPending)

	errChan := make(chan error)
	if err := batcher.Start(errChan); err != nil {
		log.Fatalf("failed to start indexing batcher: %s", err.Error())
	}
	log.Printf("batching configured with size %d, timeout %s, max pending %d",
		cfg.BatchSize, cfg.BatchTimeout, cfg.IndexMaxPending)

	// Start draining batcher errors.
	go drainLog("error indexing batch", errChan)

	// Start the ingest collectors (TCP with optional TLS, UDP) if requested.
	certReload, err := startIngestCollectors(cfg, batcher, &components)
	if err != nil {
		log.Fatal(err)
	}

	// Start profiling.
	startProfile(cfg.CPUProfile, cfg.MemProfile)

	// Wait for signals; SIGHUP reloads TLS certificates, SIGINT/SIGTERM shut down.
	var reload func() error
	if certReload != nil {
		reload = certReload.Reload
	}
	waitForSignals(reload)

	// Ordered shutdown (Issue #49): stop accepting first (no new work), then
	// flush and stop the batcher, then close the engine (waits for in-flight
	// searches and retention), then stop profiling.
	components.stop()
	batcher.Stop()
	engine.Close()

	stopProfile()
	log.Println("shutdown complete")
}
