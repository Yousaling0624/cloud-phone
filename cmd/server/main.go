// Command cloud-phone-server runs the platform's HTTP backend on its own.
// The desktop client can point at it (client.json) so several terminals
// share one database.
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"cloud-phone/internal/clientconfig"
	"cloud-phone/internal/host"
)

const version = "0.8.3"

func main() {
	var (
		listen   = flag.String("listen", "127.0.0.1:8787", "HTTP listen address")
		token    = flag.String("token", os.Getenv("CLOUDPHONE_TOKEN"), "bearer token clients must present")
		db       = flag.String("db", "", "SQLite database path (default: user config dir)")
		seed     = flag.Bool("seed", false, "insert demo rows when the database is empty (demos/tests only)")
		demo     = flag.Bool("demo", true, "run the built-in provider gateway")
		failRate = flag.Float64("demo-fail-rate", 0, "demo login failure rate, 0..1")
	)
	flag.Parse()

	if *token == "" {
		log.Fatal("必须通过 --token 或 CLOUDPHONE_TOKEN 设置访问令牌")
	}
	dbPath := *db
	if dbPath == "" {
		dir, err := clientconfig.DataDir()
		if err != nil {
			log.Fatal(err)
		}
		dbPath = filepath.Join(dir, "cloudphone.db")
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		log.Fatal(err)
	}

	inst, err := host.Start(host.Options{
		Listen: *listen, Token: *token, DBPath: dbPath,
		Seed: *seed, Demo: *demo, DemoFailRate: *failRate, Version: version,
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("cloud-phone-server %s listening on %s (database %s)", version, inst.URL, dbPath)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	done := make(chan error, 1)
	go func() { done <- inst.Wait() }()
	select {
	case <-ctx.Done():
		log.Printf("shutting down")
	case err := <-done:
		if err != nil {
			log.Printf("server stopped: %v", err)
		}
	}

	if err := inst.Close(); err != nil {
		log.Printf("shutdown: %v", err)
	}
}
