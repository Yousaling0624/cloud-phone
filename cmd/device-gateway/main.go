// Command device-gateway exposes physical Android devices over the
// cloud-phone provider protocol. It controls devices through adb; interactive
// screen/keyboard control stays in an external scrcpy/Escrcpy session.
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"cloud-phone/internal/provider/adbgateway"
)

const version = "0.1.0"

func main() {
	var (
		listen  = flag.String("listen", "127.0.0.1:8791", "HTTP listen address")
		token   = flag.String("token", os.Getenv("CLOUDPHONE_GATEWAY_TOKEN"), "bearer token the platform must present")
		adb     = flag.String("adb", "adb", "path to the adb executable")
		cfgPath = flag.String("config", "", "device config JSON (default: auto-discover adb devices)")
	)
	flag.Parse()

	if strings.TrimSpace(*token) == "" {
		log.Fatal("必须通过 --token 或 CLOUDPHONE_GATEWAY_TOKEN 设置网关令牌")
	}
	cfg, err := adbgateway.LoadConfig(*cfgPath)
	if err != nil {
		log.Fatal(err)
	}

	runner := adbgateway.NewExecRunner(*adb)
	gw, err := adbgateway.New(adbgateway.Options{
		Runner: runner, Token: *token, Devices: cfg,
		ADBName: *adb, Logger: log.Default(),
	})
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	devices, adbErr := runner.Devices(ctx)
	cancel()
	if adbErr != nil {
		log.Printf("警告: 无法执行 %s devices: %v", *adb, adbErr)
	} else {
		log.Printf("ADB 已连接，发现 %d 台设备", len(devices))
	}

	srv := &http.Server{
		Addr:              *listen,
		Handler:           gw.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       120 * time.Second,
		// No WriteTimeout: screenshot and discovery can take a few seconds.
	}
	done := make(chan error, 1)
	go func() {
		log.Printf("device-gateway %s listening on %s (adb %s)", version, *listen, *adb)
		done <- srv.ListenAndServe()
	}()

	stop, stopSignal := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stopSignal()
	select {
	case <-stop.Done():
		log.Printf("shutting down")
	case err := <-done:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
		return
	}

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}
