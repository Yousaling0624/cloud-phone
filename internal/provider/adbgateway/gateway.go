package adbgateway

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// Connectivity states used in GET /v1/phones. They describe whether adb can
// reach the device; the platform maps them onto its own phone states.
const (
	StateOnline       = "online"
	StateOffline      = "offline"
	StateUnauthorized = "unauthorized"
)

const (
	Version       = "0.1.0"
	maxBodyBytes  = 1 << 20
	maxScreenshot = 8 << 20
)

// PhoneInfo is one device in the discovery response.
type PhoneInfo struct {
	ExternalID string `json:"external_id"`
	Name       string `json:"name"`
	Region     string `json:"region"`
	Image      string `json:"image"`
	State      string `json:"state"`
	IP         string `json:"ip"`
}

// Options wires a Gateway. Runner and Token are required.
type Options struct {
	Runner  Runner
	Token   string
	Devices Config
	ADBName string
	Logger  *log.Logger
}

// Gateway serves the provider protocol for physical Android devices.
type Gateway struct {
	runner  Runner
	token   string
	devices Config
	adbName string
	logger  *log.Logger
}

func New(opts Options) (*Gateway, error) {
	if opts.Runner == nil {
		return nil, errors.New("device gateway: runner is required")
	}
	if strings.TrimSpace(opts.Token) == "" {
		return nil, errors.New("device gateway: token is required")
	}
	adbName := strings.TrimSpace(opts.ADBName)
	if adbName == "" {
		adbName = "adb"
	}
	logger := opts.Logger
	if logger == nil {
		logger = log.Default()
	}
	return &Gateway{
		runner: opts.Runner, token: opts.Token, devices: opts.Devices,
		adbName: adbName, logger: logger,
	}, nil
}

// Handler returns the authenticated HTTP API.
func (g *Gateway) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/health", g.handleHealth)
	mux.HandleFunc("GET /v1/phones", g.handleListPhones)
	mux.HandleFunc("POST /v1/phones/{id}/power", g.handlePower)
	mux.HandleFunc("GET /v1/phones/{id}/screenshot", g.handleScreenshot)
	mux.HandleFunc("POST /v1/login/execute", g.handleLogin)
	mux.HandleFunc("GET /v1/sms/wait", g.handleSMSUnsupported)
	return g.auth(mux)
}

func (g *Gateway) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := r.Header.Get("Authorization")
		const prefix = "Bearer "
		if !strings.HasPrefix(got, prefix) ||
			subtle.ConstantTimeCompare([]byte(got[len(prefix):]), []byte(g.token)) != 1 {
			writeError(w, http.StatusUnauthorized, "unauthorized", "认证失败：请检查网关令牌")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// --- discovery ---

type resolved struct {
	info     PhoneInfo
	selector string
}

func (g *Gateway) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	devices, err := g.runner.Devices(ctx)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"ok": false, "version": Version, "adb": g.adbName, "error": err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "version": Version, "adb": g.adbName, "devices": len(devices),
	})
}

func (g *Gateway) handleListPhones(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	devices, err := g.resolve(ctx, true)
	if err != nil {
		writeError(w, http.StatusBadGateway, "adb", "读取 ADB 设备失败: "+err.Error())
		return
	}
	out := make([]PhoneInfo, 0, len(devices))
	for _, d := range devices {
		out = append(out, d.info)
	}
	writeJSON(w, http.StatusOK, map[string]any{"phones": out})
}

// resolve lists configured devices plus every attached adb device not already
// represented. Configured TCP endpoints are connected first if missing.
func (g *Gateway) resolve(ctx context.Context, details bool) ([]resolved, error) {
	adbDevices, err := g.runner.Devices(ctx)
	if err != nil {
		return nil, err
	}
	adbDevices, err = g.connectMissing(ctx, adbDevices)
	if err != nil {
		return nil, err
	}

	bySerial := make(map[string]ADBDevice, len(adbDevices))
	for _, d := range adbDevices {
		bySerial[d.Serial] = d
	}
	used := make(map[string]bool, len(adbDevices))
	out := make([]resolved, 0, len(adbDevices)+len(g.devices.Devices))

	for _, cfg := range g.devices.Devices {
		selector := cfg.Selector()
		infoCfg := cfg
		d, ok := bySerial[selector]
		if !ok && cfg.Address != "" && cfg.Serial != "" {
			// The TCP endpoint is gone but the device is still attached by
			// USB; keep it usable instead of reporting it offline.
			if usb, found := bySerial[cfg.Serial]; found {
				selector, d, ok = cfg.Serial, usb, true
				infoCfg.Address = ""
			}
		}
		used[selector] = true
		if cfg.Serial != "" {
			used[cfg.Serial] = true
		}
		var dptr *ADBDevice
		if ok {
			dptr = &d
		}
		out = append(out, resolved{
			info:     g.phoneInfo(ctx, infoCfg, dptr, selector, details),
			selector: selector,
		})
	}

	var extras []ADBDevice
	for _, d := range adbDevices {
		if !used[d.Serial] {
			extras = append(extras, d)
		}
	}
	sort.Slice(extras, func(i, j int) bool { return extras[i].Serial < extras[j].Serial })
	for _, d := range extras {
		dd := d
		out = append(out, resolved{
			info:     g.phoneInfo(ctx, DeviceConfig{}, &dd, d.Serial, details),
			selector: d.Serial,
		})
	}
	return out, nil
}

func (g *Gateway) connectMissing(ctx context.Context, list []ADBDevice) ([]ADBDevice, error) {
	present := make(map[string]bool, len(list))
	for _, d := range list {
		present[d.Serial] = true
	}
	var wg sync.WaitGroup
	attempted := false
	for _, cfg := range g.devices.Devices {
		if cfg.Address == "" || present[cfg.Address] {
			continue
		}
		attempted = true
		address := cfg.Address
		wg.Add(1)
		go func() {
			defer wg.Done()
			cctx, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()
			if err := g.runner.Connect(cctx, address); err != nil {
				g.logger.Printf("adb connect %s: %v", address, err)
			}
		}()
	}
	wg.Wait()
	if !attempted {
		return list, nil
	}
	refreshed, err := g.runner.Devices(ctx)
	if err != nil {
		// The first listing worked; keep it rather than failing the sync
		// because a reconnect raced with adb.
		g.logger.Printf("adb devices after connect: %v", err)
		return list, nil
	}
	return refreshed, nil
}

func (g *Gateway) phoneInfo(ctx context.Context, cfg DeviceConfig, d *ADBDevice, selector string, details bool) PhoneInfo {
	info := PhoneInfo{
		ExternalID: cfg.ExternalID(),
		Name:       cfg.Name,
		Region:     cfg.Region,
		Image:      cfg.Image,
		State:      StateOffline,
		IP:         cfg.Address,
	}
	if info.ExternalID == "" && d != nil {
		info.ExternalID = d.Serial
	}
	if info.Region == "" {
		info.Region = "本地"
	}
	if d != nil {
		info.State = connectivity(d.State)
	}
	if info.IP == "" && d != nil && strings.Contains(d.Serial, ":") {
		info.IP = d.Serial
	}
	if info.Name == "" {
		info.Name = friendlyName(d, info.ExternalID)
	}
	if info.Image == "" {
		if details && info.State == StateOnline {
			info.Image = g.androidVersion(ctx, selector)
		}
		if info.Image == "" {
			info.Image = "Android"
		}
	}
	return info
}

func connectivity(adbState string) string {
	switch adbState {
	case "device":
		return StateOnline
	case "unauthorized":
		return StateUnauthorized
	default:
		return StateOffline
	}
}

func friendlyName(d *ADBDevice, id string) string {
	model := ""
	if d != nil {
		model = strings.TrimSpace(strings.ReplaceAll(d.Model, "_", " "))
	}
	if model == "" {
		model = "Android"
	}
	suffix := id
	if len(suffix) > 4 {
		suffix = suffix[len(suffix)-4:]
	}
	if suffix == "" {
		return model
	}
	return model + "-" + suffix
}

func (g *Gateway) androidVersion(ctx context.Context, selector string) string {
	cctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	out, err := g.runner.Run(cctx, selector, "shell", "getprop", "ro.build.version.release")
	if err != nil {
		return ""
	}
	v := strings.TrimSpace(string(out))
	if v == "" {
		return ""
	}
	return "Android " + v
}

// --- power ---

func (g *Gateway) handlePower(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Action string `json:"action"`
	}
	if err := decodeBody(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	action := strings.TrimSpace(req.Action)
	switch action {
	case "start", "stop", "restart":
	default:
		writeError(w, http.StatusBadRequest, "bad_request", "未知操作: "+action)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	d, err := g.find(ctx, r.PathValue("id"))
	if err != nil {
		writeResolveError(w, err)
		return
	}
	if d.info.State != StateOnline {
		writeError(w, http.StatusConflict, "offline", "设备离线或未授权，请检查 ADB 连接")
		return
	}

	switch action {
	case "start":
		if _, err := g.runner.Run(ctx, d.selector, "shell", "input", "keyevent", "224"); err != nil {
			writeADBError(w, err)
			return
		}
		// Best effort: a secure lock still needs the human to unlock.
		if _, err := g.runner.Run(ctx, d.selector, "shell", "wm", "dismiss-keyguard"); err != nil {
			g.logger.Printf("dismiss keyguard on %s: %v", d.info.ExternalID, err)
		}
		writeJSON(w, http.StatusOK, map[string]string{"state": "running"})
	case "stop":
		if _, err := g.runner.Run(ctx, d.selector, "shell", "input", "keyevent", "223"); err != nil {
			writeADBError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"state": "stopped"})
	case "restart":
		if _, err := g.runner.Run(ctx, d.selector, "reboot"); err != nil {
			writeADBError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"state": "starting"})
	}
}

// --- screenshot ---

func (g *Gateway) handleScreenshot(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	d, err := g.find(ctx, r.PathValue("id"))
	if err != nil {
		writeResolveError(w, err)
		return
	}
	if d.info.State != StateOnline {
		writeError(w, http.StatusConflict, "offline", "设备离线或未授权，请检查 ADB 连接")
		return
	}
	data, err := g.runner.Run(ctx, d.selector, "exec-out", "screencap", "-p")
	if err != nil {
		writeADBError(w, err)
		return
	}
	if len(data) > maxScreenshot {
		writeError(w, http.StatusBadGateway, "too_large", "截图超过 8 MiB，请降低设备分辨率")
		return
	}
	if !isPNG(data) {
		writeError(w, http.StatusBadGateway, "bad_image", "adb 返回的不是 PNG 截图")
		return
	}
	h := w.Header()
	h.Set("Content-Type", "image/png")
	h.Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func isPNG(data []byte) bool {
	sig := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
	return len(data) >= len(sig) && string(data[:len(sig)]) == string(sig)
}

// --- login and SMS ---

type loginBody struct {
	Rule        string `json:"rule"`
	AccountID   int64  `json:"account_id"`
	Number      string `json:"number"`
	Display     string `json:"display"`
	VerifyPhone string `json:"verify_phone"`
	Phone       string `json:"phone"`
}

func (g *Gateway) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginBody
	if err := decodeBody(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	phone := strings.TrimSpace(req.Phone)
	if phone == "" {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok": false, "manual": false, "message": "账号未绑定云机",
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	d, err := g.find(ctx, phone)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok": false, "manual": false, "message": "没有找到绑定设备: " + phone,
		})
		return
	}
	if d.info.State != StateOnline {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok": false, "manual": false, "message": "设备离线或未授权，请检查 ADB 连接",
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":     true,
		"manual": true,
		"message": fmt.Sprintf("请使用 scrcpy -s %s 完成 QQ 登录，然后回到控制台确认",
			d.info.ExternalID),
	})
}

func (g *Gateway) handleSMSUnsupported(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not_implemented",
		"本网关未接入短信验证码，请使用真实接码手机人工输入")
}

// --- helpers ---

var errDeviceNotFound = errors.New("云机设备不存在")

func (g *Gateway) find(ctx context.Context, id string) (resolved, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	devices, err := g.resolve(ctx, false)
	if err != nil {
		return resolved{}, err
	}
	id = strings.TrimSpace(id)
	for _, d := range devices {
		if d.info.ExternalID == id {
			return d, nil
		}
	}
	// A login request may carry the display name when the platform's phone
	// has no external id. Match it before giving up.
	for _, d := range devices {
		if d.info.Name == id {
			return d, nil
		}
	}
	return resolved{}, errDeviceNotFound
}

func writeResolveError(w http.ResponseWriter, err error) {
	if errors.Is(err, errDeviceNotFound) {
		writeError(w, http.StatusNotFound, "not_found", err.Error())
		return
	}
	writeError(w, http.StatusBadGateway, "adb", "读取 ADB 设备失败: "+err.Error())
}

func writeADBError(w http.ResponseWriter, err error) {
	writeError(w, http.StatusBadGateway, "adb", "ADB 命令失败: "+err.Error())
}

func decodeBody(w http.ResponseWriter, r *http.Request, out any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(out); err != nil {
		return fmt.Errorf("请求内容无效: %v", err)
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return errors.New("请求内容只能包含一个 JSON 对象")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]string{"error": msg, "code": code})
}
