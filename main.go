// Command cloud-phone is the 存号网络 console: a Go backend that owns the
// data and the platform integrations, shown through a React page in the
// system web view. The Go side is bound to the page as the API service;
// `go tool mygo generate` writes the typed client the page imports.
package main

import (
	"context"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"cloud-phone/internal/api"
	"cloud-phone/internal/backend"
	"cloud-phone/internal/clientconfig"
	"cloud-phone/internal/host"

	"github.com/egoist/mygo"
)

const appVersion = "0.8.3"

func main() {
	applyWSLRenderFallbacks()

	app := api.New()
	mygo.Bind(app)

	var (
		closeOnce  sync.Once
		inst       *host.Instance
		stopEvents context.CancelFunc
	)
	shutdown := func() {
		closeOnce.Do(func() {
			if stopEvents != nil {
				stopEvents()
			}
			if inst != nil {
				if err := inst.Close(); err != nil {
					log.Printf("close embedded server: %v", err)
				}
			}
		})
	}

	mygo.App.WhenReady(func() {
		cfgPath, cfg, forceEmbedded, cfgErr := loadClientConfig()

		var (
			be        backend.Backend
			mode      = "embedded"
			serverURL string
			startErr  = cfgErr
		)
		if cfgErr == nil {
			if forceEmbedded {
				cfg.ServerURL = ""
			}
			if cfg.ServerURL == "" {
				mode = "embedded"
				var err error
				be, inst, err = startEmbedded()
				if err != nil {
					startErr = err
				} else {
					serverURL = inst.URL
				}
			} else {
				mode = "remote"
				serverURL = cfg.ServerURL
				var err error
				be, err = backend.NewRemote(cfg.ServerURL, cfg.Token)
				if err != nil {
					startErr = err
				}
			}
		}

		api.Configure(app, api.Config{
			ConfigPath: cfgPath,
			Connection: api.ConnectionInfo{Mode: mode, ServerURL: serverURL},
		})
		if startErr != nil {
			log.Printf("connect backend: %v", startErr)
			api.SetStartupError(app, startErr)
		}
		if be != nil {
			api.AttachBackend(app, be)
			if sub, ok := be.(backend.Subscriber); ok {
				ctx, cancel := context.WithCancel(context.Background())
				stopEvents = cancel
				go func() {
					if err := sub.Subscribe(ctx, api.EmitChange); err != nil {
						log.Printf("event stream: %v", err)
					}
				}()
			}
		}

		mygo.NewWindow(mygo.WindowOptions{
			Title:           "存号网络 · 云手机运营平台",
			Width:           1360,
			Height:          860,
			MinWidth:        1040,
			MinHeight:       680,
			BackgroundColor: "#f4f6fb", // the page's --canvas, so the first frame is not black
			StateKey:        "main",    // reopens where the user left it
			URL:             "/",       // the embedded frontend served by mygo
			// The page draws the title bar: the topbar and the sidebar's brand
			// are drag regions, and the native window controls sit over them.
			TitleBarStyle:  mygo.TitleBarHiddenInset,
			TitleBarHeight: 60,
		})
	})

	mygo.App.OnQuit(shutdown)
	if err := mygo.App.Run(); err != nil {
		log.Print(err)
	}
	shutdown()
}

// loadClientConfig reads client.json and applies environment overrides.
// CLOUDPHONE_EMBEDDED=1 forces the built-in server even when an address is
// configured, which keeps tests and one-off runs isolated from the file.
func loadClientConfig() (path string, cfg clientconfig.Config, embedded bool, err error) {
	path, err = clientconfig.DefaultPath()
	if err != nil {
		return "", cfg, false, err
	}
	cfg, err = clientconfig.Load(path)
	if err != nil {
		return path, cfg, false, err
	}
	if v := strings.TrimSpace(os.Getenv("CLOUDPHONE_SERVER_URL")); v != "" {
		cfg.ServerURL = v
	}
	if v := strings.TrimSpace(os.Getenv("CLOUDPHONE_TOKEN")); v != "" {
		cfg.Token = v
	}
	embedded = strings.TrimSpace(os.Getenv("CLOUDPHONE_EMBEDDED")) == "1"
	return path, cfg, embedded, nil
}

func startEmbedded() (backend.Backend, *host.Instance, error) {
	dir, err := clientconfig.DataDir()
	if err != nil {
		return nil, nil, err
	}
	dbPath := filepath.Join(dir, "cloudphone.db")
	adoptLegacyDatabase(dbPath)
	// A real installation starts empty. Demo rows are opt-in so a fresh
	// database never mixes sample accounts into an operator's data.
	seedDemo := strings.TrimSpace(os.Getenv("CLOUDPHONE_SEED")) == "1"
	inst, err := host.Start(host.Options{
		DBPath: dbPath, Seed: seedDemo, Demo: true, Version: appVersion,
	})
	if err != nil {
		return nil, nil, err
	}
	go func() {
		if err := inst.Wait(); err != nil {
			log.Printf("embedded server stopped: %v", err)
		}
	}()
	return inst.Backend, inst, nil
}

// applyWSLRenderFallbacks keeps WebKitGTK off a GPU path that black-screens
// under WSLg, where the window paints only where the mouse moves. Explicit
// user settings win, and other desktops are untouched.
func applyWSLRenderFallbacks() {
	if !isWSL() {
		return
	}
	if os.Getenv("WEBKIT_DISABLE_DMABUF_RENDERER") == "" {
		_ = os.Setenv("WEBKIT_DISABLE_DMABUF_RENDERER", "1")
	}
}

func isWSL() bool {
	if os.Getenv("WSL_DISTRO_NAME") != "" {
		return true
	}
	b, err := os.ReadFile("/proc/version")
	return err == nil && strings.Contains(strings.ToLower(string(b)), "microsoft")
}

// adoptLegacyDatabase copies the database the native-UI build kept in the
// working directory into the platform's user-data directory, so switching
// to the web build does not reset the user's accounts.
func adoptLegacyDatabase(newPath string) {
	if _, err := os.Stat(newPath); err == nil {
		return // already have one
	}
	oldPath := filepath.Join("userData", "cloudphone.db")
	if _, err := os.Stat(oldPath); err != nil {
		return
	}
	if err := copyFile(oldPath, newPath); err != nil {
		log.Printf("migrate legacy database: %v", err)
		return
	}
	log.Printf("migrated legacy database from %s", oldPath)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
