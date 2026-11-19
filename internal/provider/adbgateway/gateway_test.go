package adbgateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type runCall struct {
	Selector string
	Args     string
}

type fakeRunner struct {
	devices    []ADBDevice
	devicesErr error
	connectErr error
	runErr     error
	calls      []runCall
	connected  []string
}

func (f *fakeRunner) Devices(context.Context) ([]ADBDevice, error) {
	if f.devicesErr != nil {
		return nil, f.devicesErr
	}
	return f.devices, nil
}

func (f *fakeRunner) Connect(_ context.Context, address string) error {
	f.connected = append(f.connected, address)
	return f.connectErr
}

func (f *fakeRunner) Run(_ context.Context, selector string, args ...string) ([]byte, error) {
	joined := strings.Join(args, " ")
	f.calls = append(f.calls, runCall{Selector: selector, Args: joined})
	if f.runErr != nil {
		return nil, f.runErr
	}
	switch joined {
	case "shell getprop ro.build.version.release":
		return []byte("13\n"), nil
	case "exec-out screencap -p":
		return testPNG(), nil
	default:
		return []byte("ok\n"), nil
	}
}

func testPNG() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{10, 20, 30, 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func newGateway(t *testing.T, runner Runner, cfg Config) http.Handler {
	t.Helper()
	gw, err := New(Options{
		Runner: runner, Token: "secret", Devices: cfg, ADBName: "adb",
		Logger: log.New(io.Discard, "", 0),
	})
	if err != nil {
		t.Fatal(err)
	}
	return gw.Handler()
}

func request(t *testing.T, h http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, path, nil)
	} else {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func decode[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var out T
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode %q: %v", w.Body.String(), err)
	}
	return out
}

func TestAuthRequired(t *testing.T) {
	h := newGateway(t, &fakeRunner{}, Config{})
	for _, token := range []string{"", "wrong"} {
		w := request(t, h, http.MethodGet, "/v1/phones", token, "")
		if w.Code != http.StatusUnauthorized {
			t.Errorf("token %q: status = %d, want 401", token, w.Code)
		}
	}
}

func TestListPhonesConfiguredAndAuto(t *testing.T) {
	runner := &fakeRunner{devices: []ADBDevice{
		{Serial: "USB-1", State: "device", Model: "Pixel_7"},
		{Serial: "auto-1", State: "device", Model: "SM_G991B"},
		{Serial: "off-1", State: "offline"},
	}}
	cfg := Config{Devices: []DeviceConfig{
		{Serial: "USB-1", Name: "QQ-01", Region: "桌面", Image: "Android 14"},
	}}
	h := newGateway(t, runner, cfg)
	w := request(t, h, http.MethodGet, "/v1/phones", "secret", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", w.Code, w.Body.String())
	}
	out := decode[struct {
		Phones []PhoneInfo `json:"phones"`
	}](t, w)
	if len(out.Phones) != 3 {
		t.Fatalf("phones = %+v, want 3", out.Phones)
	}
	if out.Phones[0].ExternalID != "USB-1" || out.Phones[0].Name != "QQ-01" ||
		out.Phones[0].State != StateOnline || out.Phones[0].Image != "Android 14" {
		t.Errorf("configured phone = %+v", out.Phones[0])
	}
	if out.Phones[1].ExternalID != "auto-1" || out.Phones[1].State != StateOnline ||
		out.Phones[1].Name != "SM G991B-to-1" {
		t.Errorf("auto phone = %+v", out.Phones[1])
	}
	if out.Phones[2].State != StateOffline {
		t.Errorf("offline phone = %+v", out.Phones[2])
	}
}

func TestListPhonesADBFailure(t *testing.T) {
	h := newGateway(t, &fakeRunner{devicesErr: fmt.Errorf("adb missing")}, Config{})
	w := request(t, h, http.MethodGet, "/v1/phones", "secret", "")
	if w.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", w.Code)
	}
}

func TestPowerCommands(t *testing.T) {
	runner := &fakeRunner{devices: []ADBDevice{{Serial: "s1", State: "device", Model: "Pixel"}}}
	h := newGateway(t, runner, Config{})

	cases := []struct {
		action string
		want   string
		state  string
	}{
		{"start", "shell input keyevent 224", "running"},
		{"stop", "shell input keyevent 223", "stopped"},
		{"restart", "reboot", "starting"},
	}
	for _, tc := range cases {
		w := request(t, h, http.MethodPost, "/v1/phones/s1/power", "secret",
			fmt.Sprintf(`{"action":%q}`, tc.action))
		if w.Code != http.StatusOK {
			t.Fatalf("%s status = %d: %s", tc.action, w.Code, w.Body.String())
		}
		out := decode[map[string]string](t, w)
		if out["state"] != tc.state {
			t.Errorf("%s state = %q, want %q", tc.action, out["state"], tc.state)
		}
	}
	found := map[string]bool{}
	for _, c := range runner.calls {
		found[c.Args] = true
	}
	for _, tc := range cases {
		if !found[tc.want] {
			t.Errorf("missing adb call %q in %+v", tc.want, runner.calls)
		}
	}
}

func TestPowerOfflineDevice(t *testing.T) {
	h := newGateway(t, &fakeRunner{}, Config{})
	w := request(t, h, http.MethodPost, "/v1/phones/missing/power", "secret", `{"action":"start"}`)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestScreenshot(t *testing.T) {
	runner := &fakeRunner{devices: []ADBDevice{{Serial: "s1", State: "device"}}}
	h := newGateway(t, runner, Config{})
	w := request(t, h, http.MethodGet, "/v1/phones/s1/screenshot", "secret", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "image/png" {
		t.Errorf("content-type = %q", ct)
	}
	if !isPNG(w.Body.Bytes()) {
		t.Error("body is not a PNG")
	}
	if cc := w.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("cache-control = %q", cc)
	}
}

func TestScreenshotBadImage(t *testing.T) {
	runner := &fakeRunner{
		devices: []ADBDevice{{Serial: "s1", State: "device"}},
		runErr:  fmt.Errorf("boom"),
	}
	h := newGateway(t, runner, Config{})
	w := request(t, h, http.MethodGet, "/v1/phones/s1/screenshot", "secret", "")
	if w.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", w.Code)
	}
}

func TestLoginManual(t *testing.T) {
	runner := &fakeRunner{devices: []ADBDevice{{Serial: "s1", State: "device"}}}
	h := newGateway(t, runner, Config{})

	w := request(t, h, http.MethodPost, "/v1/login/execute", "secret", `{"phone":"s1"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", w.Code, w.Body.String())
	}
	out := decode[struct {
		OK     bool   `json:"ok"`
		Manual bool   `json:"manual"`
		Msg    string `json:"message"`
	}](t, w)
	if !out.OK || !out.Manual || !strings.Contains(out.Msg, "scrcpy -s s1") {
		t.Errorf("login response = %+v", out)
	}

	w = request(t, h, http.MethodPost, "/v1/login/execute", "secret", `{"phone":""}`)
	if w.Code != http.StatusOK {
		t.Fatalf("empty phone status = %d", w.Code)
	}
	if out := decode[struct {
		OK bool `json:"ok"`
	}](t, w); out.OK {
		t.Error("empty phone should not succeed")
	}

	w = request(t, h, http.MethodPost, "/v1/login/execute", "secret", `{"phone":"missing"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("unknown phone status = %d", w.Code)
	}
}

func TestSMSUnsupported(t *testing.T) {
	h := newGateway(t, &fakeRunner{}, Config{})
	w := request(t, h, http.MethodGet, "/v1/sms/wait?phone=1", "secret", "")
	if w.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, want 501", w.Code)
	}
}

func TestConfigIdentity(t *testing.T) {
	d := DeviceConfig{Serial: "USB-1", Address: "10.0.0.2:5555"}
	if d.ExternalID() != "USB-1" {
		t.Errorf("external id = %q", d.ExternalID())
	}
	if d.Selector() != "10.0.0.2:5555" {
		t.Errorf("selector = %q", d.Selector())
	}
	if _, err := LoadConfig(""); err != nil {
		t.Errorf("empty config: %v", err)
	}
}

func TestParseDevices(t *testing.T) {
	got := parseDevices("List of devices attached\nR58M123\tdevice usb:1-1 model:Pixel_7 product:panther\nbad\tunauthorized\n")
	if len(got) != 2 {
		t.Fatalf("devices = %+v", got)
	}
	if got[0].Serial != "R58M123" || got[0].Model != "Pixel_7" || got[0].State != "device" {
		t.Errorf("first = %+v", got[0])
	}
	if got[1].State != "unauthorized" {
		t.Errorf("second = %+v", got[1])
	}
}
