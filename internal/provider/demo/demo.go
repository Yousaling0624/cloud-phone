// Package demo is a local implementation of the provider gateway protocol.
// It gives the desktop app a working cloud phone, SMS inbox, and login rule
// without any external service, and doubles as the reference for a real
// vendor adapter: swappable by changing the configured base URL.
package demo

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"log"
	"math"
	mathrand "math/rand/v2"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Options configure the demo gateway.
type Options struct {
	// FailRate is the chance (0..1) that a login reports a rule failure, for
	// exercising the failure path. The default is 0 so the product works.
	FailRate float64
	// Listen is the TCP address; the default is a random loopback port.
	Listen string
	// Token is the bearer token; empty generates a random one.
	Token string
}

// Provider is a running demo gateway.
type Provider struct {
	URL   string
	Token string

	failRate  float64
	srv       *http.Server
	ln        net.Listener
	closeOnce sync.Once
}

// Start listens on a loopback port and serves until Close.
func Start(opts Options) (*Provider, error) {
	listen := opts.Listen
	if listen == "" {
		listen = "127.0.0.1:0"
	}
	ln, err := net.Listen("tcp", listen)
	if err != nil {
		return nil, fmt.Errorf("demo provider listen: %w", err)
	}
	token := opts.Token
	if token == "" {
		if token, err = randomToken(); err != nil {
			ln.Close()
			return nil, err
		}
	}
	p := &Provider{
		URL:      "http://" + ln.Addr().String(),
		Token:    token,
		failRate: opts.FailRate,
		ln:       ln,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/phones/{id}/power", p.handlePower)
	mux.HandleFunc("GET /v1/phones/{id}/screenshot", p.handleScreenshot)
	mux.HandleFunc("GET /v1/sms/wait", p.handleSMSWait)
	mux.HandleFunc("POST /v1/login/execute", p.handleLogin)
	p.srv = &http.Server{
		Handler:           p.auth(mux),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	go func() {
		if err := p.srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("demo provider: %v", err)
		}
	}()
	return p, nil
}

// Close stops the gateway.
func (p *Provider) Close() error {
	var err error
	p.closeOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		err = p.srv.Shutdown(ctx)
	})
	return err
}

func (p *Provider) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if got != p.Token {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid token"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (p *Provider) handlePower(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Action string `json:"action"`
	}
	if !decode(w, r, &req) {
		return
	}
	var d time.Duration
	state := "running"
	switch req.Action {
	case "start":
		d = 600 * time.Millisecond
	case "stop":
		d = 300 * time.Millisecond
		state = "stopped"
	case "restart":
		d = 800 * time.Millisecond
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown action"})
		return
	}
	if err := sleepCtx(r.Context(), d); err != nil {
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"state": state, "device": r.PathValue("id")})
}

func (p *Provider) handleScreenshot(w http.ResponseWriter, r *http.Request) {
	data, err := renderPNG(time.Now())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	h := w.Header()
	h.Set("Content-Type", "image/png")
	h.Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (p *Provider) handleSMSWait(w http.ResponseWriter, r *http.Request) {
	phone := strings.TrimSpace(r.URL.Query().Get("phone"))
	if phone == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "phone is required"})
		return
	}
	if err := sleepCtx(r.Context(), smsDelay()); err != nil {
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"phone": phone, "code": randomCode()})
}

func (p *Provider) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Rule    string `json:"rule"`
		Account string `json:"number"`
		Phone   string `json:"phone"`
	}
	if !decode(w, r, &req) {
		return
	}
	if req.Rule == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "rule is required"})
		return
	}
	if p.failRate > 0 && mathrand.Float64() < p.failRate {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok": false, "message": "执行规则失败: " + req.Rule,
		})
		return
	}
	if err := sleepCtx(r.Context(), smsDelay()); err != nil {
		return
	}
	code := randomCode()
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "code": code, "message": "登录成功",
	})
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		if err == io.EOF {
			return true
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func smsDelay() time.Duration {
	return time.Duration(3+mathrand.IntN(6)) * time.Second
}

func randomCode() string {
	return fmt.Sprintf("%06d", mathrand.IntN(1_000_000))
}

func randomToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// --- screenshot rendering ---

var (
	bgTop      = color.RGBA{24, 36, 72, 255}
	bgBottom   = color.RGBA{64, 36, 96, 255}
	white      = color.RGBA{246, 248, 252, 255}
	muted      = color.RGBA{190, 200, 220, 180}
	statusBg   = color.RGBA{0, 0, 0, 90}
	iconColors = []color.RGBA{
		{52, 152, 219, 255}, {46, 204, 113, 255}, {241, 196, 15, 255},
		{231, 76, 60, 255}, {155, 89, 182, 255}, {26, 188, 156, 255},
		{230, 126, 34, 255}, {52, 73, 94, 255}, {236, 240, 241, 255},
		{142, 68, 173, 255}, {39, 174, 96, 255}, {211, 84, 0, 255},
		{41, 128, 185, 255}, {192, 57, 43, 255}, {243, 156, 18, 255},
		{30, 130, 76, 255}, {142, 68, 173, 255}, {44, 62, 80, 255},
		{52, 152, 219, 255}, {39, 174, 96, 255},
	}
)

// renderPNG draws an Android-like home screen. The clock and the moving
// accent dot change between polls, so the console visibly shows a live
// device rather than a static placeholder.
func renderPNG(now time.Time) ([]byte, error) {
	const w, h = 480, 800
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		t := float64(y) / float64(h-1)
		c := color.RGBA{
			uint8(float64(bgTop.R) + (float64(bgBottom.R)-float64(bgTop.R))*t),
			uint8(float64(bgTop.G) + (float64(bgBottom.G)-float64(bgTop.G))*t),
			uint8(float64(bgTop.B) + (float64(bgBottom.B)-float64(bgTop.B))*t),
			255,
		}
		draw.Draw(img, image.Rect(0, y, w, y+1), &image.Uniform{C: c}, image.Point{}, draw.Src)
	}

	// Status bar.
	draw.Draw(img, image.Rect(0, 0, w, 40), &image.Uniform{C: statusBg}, image.Point{}, draw.Over)
	fillRoundRect(img, 18, 24, 44, 8, 4, muted)   // signal bars
	fillRoundRect(img, 70, 18, 24, 14, 3, muted)  // wifi-ish block
	fillRoundRect(img, 410, 20, 30, 12, 3, muted) // battery
	fillRoundRect(img, 440, 23, 4, 6, 2, muted)   // battery tip

	// Clock: HH:MM as seven-segment digits.
	hh := now.Hour()
	mm := now.Minute()
	clock := fmt.Sprintf("%02d%02d", hh, mm)
	x := 96
	for i := 0; i < 4; i++ {
		drawDigit(img, x, 84, clock[i]-'0', white)
		x += 50
		if i == 1 {
			fillRoundRect(img, x-12, 96, 8, 8, 4, white)
			fillRoundRect(img, x-12, 120, 8, 8, 4, white)
			x += 18
		}
	}

	// Date line, as a row of small blocks.
	fillRoundRect(img, 150, 164, 180, 8, 4, muted)

	// App grid: 4 x 5 icons with one highlighted each few seconds.
	highlight := int(now.Unix()/3) % len(iconColors)
	for row := 0; row < 5; row++ {
		for col := 0; col < 4; col++ {
			idx := row*4 + col
			x0 := 40 + col*104
			y0 := 210 + row*104
			if idx == highlight {
				fillRoundRect(img, x0-5, y0-5, 82, 82, 20, white)
				fillRoundRect(img, x0, y0, 72, 72, 16, iconColors[idx])
			} else {
				fillRoundRect(img, x0, y0, 72, 72, 16, iconColors[idx])
			}
			// A simple glyph inside each icon.
			fillRoundRect(img, x0+20, y0+20, 32, 32, 9, color.RGBA{255, 255, 255, 200})
		}
	}

	// Dock.
	fillRoundRect(img, 20, 716, 440, 64, 28, color.RGBA{10, 16, 32, 160})
	for i := 0; i < 4; i++ {
		fillRoundRect(img, 48+i*104, 730, 44, 36, 10, iconColors[(i*3+2)%len(iconColors)])
	}

	// A dot sweeping across the screen, so two screenshots a second apart
	// differ even within the same minute.
	sweep := float64(now.UnixMilli()%6000) / 6000
	cx := 30 + int(sweep*420)
	cy := 707
	fillCircle(img, cx, cy, 6, color.RGBA{255, 255, 255, 210})

	var out writeBuffer
	enc := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := enc.Encode(&out, img); err != nil {
		return nil, err
	}
	return out.b, nil
}

type writeBuffer struct{ b []byte }

func (w *writeBuffer) Write(p []byte) (int, error) {
	w.b = append(w.b, p...)
	return len(p), nil
}

// fillRoundRect fills a rectangle with rounded corners by drawing one
// horizontal span per row.
func fillRoundRect(img *image.RGBA, x, y, w, h, r int, c color.RGBA) {
	if w <= 0 || h <= 0 {
		return
	}
	if r*2 > w {
		r = w / 2
	}
	if r*2 > h {
		r = h / 2
	}
	for yy := 0; yy < h; yy++ {
		inset := 0
		if yy < r {
			dy := float64(r-yy) - 0.5
			inset = r - int(math.Sqrt(float64(r*r)-dy*dy)+0.5)
		} else if yy >= h-r {
			dy := float64(yy-(h-r)) + 0.5
			inset = r - int(math.Sqrt(float64(r*r)-dy*dy)+0.5)
		}
		draw.Draw(img, image.Rect(x+inset, y+yy, x+w-inset, y+yy+1), &image.Uniform{C: c}, image.Point{}, draw.Over)
	}
}

func fillCircle(img *image.RGBA, cx, cy, r int, c color.RGBA) {
	for yy := -r; yy <= r; yy++ {
		span := int(math.Sqrt(float64(r*r - yy*yy)))
		draw.Draw(img, image.Rect(cx-span, cy+yy, cx+span+1, cy+yy+1), &image.Uniform{C: c}, image.Point{}, draw.Over)
	}
}

// Seven-segment digit table: segments a b c d e f g.
var digitSegments = [10][7]bool{
	{true, true, true, true, true, true, false},     // 0
	{false, true, true, false, false, false, false}, // 1
	{true, true, false, true, true, false, true},    // 2
	{true, true, true, true, false, false, true},    // 3
	{false, true, true, false, false, true, true},   // 4
	{true, false, true, true, false, true, true},    // 5
	{true, false, true, true, true, true, true},     // 6
	{true, true, true, false, false, false, false},  // 7
	{true, true, true, true, true, true, true},      // 8
	{true, true, true, true, false, true, true},     // 9
}

func drawDigit(img *image.RGBA, x, y int, d byte, c color.RGBA) {
	const (
		th = 8
		w  = 32
		h  = 56
	)
	if d > 9 {
		return
	}
	seg := digitSegments[d]
	bar := func(sx, sy, sw, sh int) { fillRoundRect(img, sx, sy, sw, sh, 3, c) }
	if seg[0] {
		bar(x, y, w, th)
	}
	if seg[1] {
		bar(x+w-th, y, th, h/2)
	}
	if seg[2] {
		bar(x+w-th, y+h/2, th, h/2)
	}
	if seg[3] {
		bar(x, y+h-th, w, th)
	}
	if seg[4] {
		bar(x, y+h/2, th, h/2)
	}
	if seg[5] {
		bar(x, y, th, h/2)
	}
	if seg[6] {
		bar(x, y+h/2-th/2, w, th)
	}
}
