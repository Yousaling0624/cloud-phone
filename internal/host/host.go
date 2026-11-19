// Package host is the composition root shared by the standalone server and
// the desktop app's embedded server: open the database, start the demo
// gateway, build the service, repair startup state, and serve HTTP.
package host

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"cloud-phone/internal/backend"
	"cloud-phone/internal/driver"
	"cloud-phone/internal/provider/demo"
	"cloud-phone/internal/server"
	"cloud-phone/internal/service"
	"cloud-phone/internal/store"
)

// Options configure one embedded or standalone instance.
type Options struct {
	Listen       string // default 127.0.0.1:0
	Token        string // empty generates a random token
	DBPath       string // required; ":memory:" is allowed for tests
	Seed         bool
	Demo         bool
	DemoFailRate float64
	Version      string
}

// Instance is a running server plus the pieces its owner may need.
type Instance struct {
	URL     string
	Token   string
	DBPath  string
	Repo    *store.SQLite
	Service *service.Service
	Server  *server.Server
	Demo    *demo.Provider
	Backend backend.Backend

	serveErr  chan error
	closeOnce sync.Once
	closeErr  error
}

// Start brings up a complete instance. On failure every resource it opened
// is closed before returning.
func Start(opts Options) (*Instance, error) {
	if opts.DBPath == "" {
		return nil, errors.New("database path is required")
	}
	if opts.Listen == "" {
		opts.Listen = "127.0.0.1:0"
	}
	token := opts.Token
	if token == "" {
		var err error
		if token, err = randomToken(); err != nil {
			return nil, err
		}
	}

	if opts.DBPath != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(opts.DBPath), 0o755); err != nil {
			return nil, fmt.Errorf("create data directory: %w", err)
		}
	}
	repo, err := store.Open(opts.DBPath)
	if err != nil {
		return nil, err
	}

	var (
		demoP *demo.Provider
		svc   *service.Service
		ln    net.Listener
	)
	ok := false
	defer func() {
		if ok {
			return
		}
		if svc != nil {
			svc.Close()
		}
		if demoP != nil {
			demoP.Close()
		}
		if ln != nil {
			ln.Close()
		}
		repo.Close()
	}()

	ctx := context.Background()
	if opts.Seed {
		if err := repo.Seed(ctx); err != nil {
			return nil, fmt.Errorf("seed database: %w", err)
		}
	}

	if opts.Demo {
		if demoP, err = demo.Start(demo.Options{FailRate: opts.DemoFailRate}); err != nil {
			return nil, err
		}
	}

	var demoURL, demoToken string
	if demoP != nil {
		demoURL, demoToken = demoP.URL, demoP.Token
	}
	gw := driver.NewGateway(repo.GetSettings, demoURL, demoToken)
	svc = service.New(repo, gw, gw, gw)

	if err := svc.RecoverStuckLogins(ctx); err != nil {
		return nil, fmt.Errorf("recover logins: %w", err)
	}
	if fixed, err := svc.RepairBindings(ctx); err != nil {
		return nil, fmt.Errorf("repair bindings: %w", err)
	} else if fixed > 0 {
		log.Printf("repaired %d account/phone binding(s)", fixed)
	}

	srv := server.New(server.Options{Service: svc, Token: token, Version: opts.Version})
	if ln, err = net.Listen("tcp", opts.Listen); err != nil {
		return nil, fmt.Errorf("listen on %s: %w", opts.Listen, err)
	}
	url := publicURL(ln.Addr())
	remote, err := backend.NewRemote(url, token)
	if err != nil {
		return nil, err
	}

	inst := &Instance{
		URL: url, Token: token, DBPath: opts.DBPath,
		Repo: repo, Service: svc, Server: srv, Demo: demoP,
		Backend:  remote,
		serveErr: make(chan error, 1),
	}
	go func() {
		inst.serveErr <- srv.Serve(ln)
	}()
	ok = true
	return inst, nil
}

// Wait reports why serving stopped; a clean shutdown returns nil.
func (i *Instance) Wait() error {
	return <-i.serveErr
}

// Close shuts the SSE streams, the HTTP server, the login workers, the demo
// gateway, and the database, in that order. Safe to call more than once.
func (i *Instance) Close() error {
	i.closeOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		var errs []error
		if i.Server != nil {
			if err := i.Server.Shutdown(ctx); err != nil {
				errs = append(errs, err)
			}
		}
		if i.Service != nil {
			i.Service.Close()
		}
		if i.Demo != nil {
			if err := i.Demo.Close(); err != nil {
				errs = append(errs, err)
			}
		}
		if i.Repo != nil {
			if err := i.Repo.Close(); err != nil {
				errs = append(errs, err)
			}
		}
		i.closeErr = errors.Join(errs...)
	})
	return i.closeErr
}

// publicURL turns a listener address into a usable URL, replacing an
// unspecified host with loopback.
func publicURL(addr net.Addr) string {
	host, port, err := net.SplitHostPort(addr.String())
	if err != nil {
		return "http://" + addr.String()
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	return "http://" + net.JoinHostPort(host, port)
}

func randomToken() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return hex.EncodeToString(b), nil
}
