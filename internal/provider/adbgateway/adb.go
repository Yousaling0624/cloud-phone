package adbgateway

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// ADBDevice is one row from `adb devices -l`.
type ADBDevice struct {
	Serial  string
	State   string // device, offline, unauthorized, ...
	Model   string
	Product string
}

// Runner is the small part of adb the gateway needs. Tests substitute a
// fake; the real implementation shells out to the adb binary.
type Runner interface {
	Devices(ctx context.Context) ([]ADBDevice, error)
	Connect(ctx context.Context, address string) error
	Run(ctx context.Context, selector string, args ...string) ([]byte, error)
}

// ExecRunner runs the adb executable directly, never through a shell.
type ExecRunner struct {
	Path string
}

func NewExecRunner(path string) *ExecRunner {
	if strings.TrimSpace(path) == "" {
		path = "adb"
	}
	return &ExecRunner{Path: path}
}

func (r *ExecRunner) output(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, r.Path, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(string(out))
		}
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("adb %s: %s", strings.Join(args, " "), msg)
	}
	return out, nil
}

func (r *ExecRunner) Devices(ctx context.Context) ([]ADBDevice, error) {
	out, err := r.output(ctx, "devices", "-l")
	if err != nil {
		return nil, err
	}
	return parseDevices(string(out)), nil
}

func parseDevices(text string) []ADBDevice {
	var out []ADBDevice
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "*") || strings.EqualFold(line, "List of devices attached") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		d := ADBDevice{Serial: fields[0], State: fields[1]}
		if d.State == "no" {
			// "no permissions (user in plugdev group); see [...]"
			d.State = "unauthorized"
		}
		for _, f := range fields[2:] {
			switch {
			case strings.HasPrefix(f, "model:"):
				d.Model = strings.TrimPrefix(f, "model:")
			case strings.HasPrefix(f, "product:"):
				d.Product = strings.TrimPrefix(f, "product:")
			}
		}
		out = append(out, d)
	}
	return out
}

func (r *ExecRunner) Connect(ctx context.Context, address string) error {
	out, err := r.output(ctx, "connect", address)
	if err != nil {
		return err
	}
	msg := strings.ToLower(strings.TrimSpace(string(out)))
	if strings.Contains(msg, "failed") || strings.Contains(msg, "cannot") || strings.Contains(msg, "unable") {
		return fmt.Errorf("adb connect %s: %s", address, strings.TrimSpace(string(out)))
	}
	return nil
}

func (r *ExecRunner) Run(ctx context.Context, selector string, args ...string) ([]byte, error) {
	full := append([]string{"-s", selector}, args...)
	return r.output(ctx, full...)
}
