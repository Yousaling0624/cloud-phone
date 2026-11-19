// Package server is the standalone HTTP surface of the platform: it serves
// the snapshot, the mutations, the screenshot stream, and the SSE change
// feed. The desktop app embeds the same server when no remote is configured,
// and cmd/server runs it on its own.
package server

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"cloud-phone/internal/driver"
	"cloud-phone/internal/model"
	"cloud-phone/internal/service"
)

// Version is reported by /api/health.
const Version = "0.8.3"

const (
	maxBody     = 1 << 20 // ordinary JSON requests
	maxCSVInput = 4 << 20 // pasted CSV files
)

// Options configure a Server.
type Options struct {
	Service *service.Service
	Token   string
	Version string
}

// Server wraps the service in an HTTP handler and owns the SSE hub.
type Server struct {
	svc     *service.Service
	hub     *Hub
	token   string
	version string
	handler http.Handler

	mu      sync.Mutex
	httpSrv *http.Server
}

// New builds the handler and attaches the change notifier.
func New(opts Options) *Server {
	version := opts.Version
	if version == "" {
		version = Version
	}
	s := &Server{
		svc:     opts.Service,
		hub:     NewHub(),
		token:   strings.TrimSpace(opts.Token),
		version: version,
	}
	mux := http.NewServeMux()
	s.routes(mux)
	s.handler = s.auth(mux)
	s.svc.SetNotifier(s.hub.Publish)
	return s
}

// Handler returns the authenticated HTTP handler.
func (s *Server) Handler() http.Handler { return s.handler }

// Serve starts serving on ln. It returns nil after a clean Shutdown.
func (s *Server) Serve(ln net.Listener) error {
	srv := &http.Server{
		Handler:           s.handler,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       120 * time.Second,
		// No WriteTimeout: it would cut every SSE stream.
	}
	s.mu.Lock()
	s.httpSrv = srv
	s.mu.Unlock()
	err := srv.Serve(ln)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// Shutdown closes the event streams first, then drains HTTP. Canceling the
// hub releases the SSE handlers that Shutdown would otherwise wait for.
func (s *Server) Shutdown(ctx context.Context) error {
	s.hub.Close()
	s.svc.SetNotifier(nil)
	s.mu.Lock()
	srv := s.httpSrv
	s.mu.Unlock()
	if srv == nil {
		return nil
	}
	return srv.Shutdown(ctx)
}

func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.token != "" {
			got := r.Header.Get("Authorization")
			const prefix = "Bearer "
			if !strings.HasPrefix(got, prefix) ||
				subtle.ConstantTimeCompare([]byte(got[len(prefix):]), []byte(s.token)) != 1 {
				writeError(w, http.StatusUnauthorized, "unauthorized", "认证失败：请检查访问令牌")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/snapshot", s.handleSnapshot)
	mux.HandleFunc("GET /api/events", s.handleEvents)

	mux.HandleFunc("POST /api/accounts", s.handleCreateAccount)
	mux.HandleFunc("PATCH /api/accounts/{id}", s.handleUpdateAccount)
	mux.HandleFunc("DELETE /api/accounts/{id}", s.handleDeleteAccount)
	mux.HandleFunc("POST /api/accounts/batch-delete", s.handleBatchDelete)
	mux.HandleFunc("POST /api/accounts/import", s.handleImportCSV)
	mux.HandleFunc("POST /api/accounts/export.csv", s.handleExportCSV)
	mux.HandleFunc("POST /api/accounts/{id}/login", s.handleStartLogin)
	mux.HandleFunc("POST /api/accounts/{id}/login/confirm", s.handleConfirmLogin)
	mux.HandleFunc("GET /api/accounts/{id}/screenshot", s.handleScreenshot)

	mux.HandleFunc("POST /api/phones/{id}/state", s.handlePhoneState)
	mux.HandleFunc("POST /api/phones/sync", s.handleSyncPhones)
	mux.HandleFunc("POST /api/users/{id}/enabled", s.handleUserEnabled)
	mux.HandleFunc("POST /api/wallet/topup", s.handleTopUp)
	mux.HandleFunc("POST /api/wallet/charge", s.handleCharge)
	mux.HandleFunc("GET /api/settings", s.handleGetSettings)
	mux.HandleFunc("PUT /api/settings", s.handleSaveSettings)
}

// --- read ---

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	status := "ok"
	code := http.StatusOK
	if _, err := s.svc.Repo().Wallet(ctx); err != nil {
		status = err.Error()
		code = http.StatusInternalServerError
	}
	writeJSON(w, code, map[string]any{
		"ok":       code == http.StatusOK,
		"version":  s.version,
		"database": status,
	})
}

func (s *Server) handleSnapshot(w http.ResponseWriter, r *http.Request) {
	snap, err := s.svc.Snapshot(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "no_stream", "当前连接不支持流式响应")
		return
	}
	h := w.Header()
	h.Set("Content-Type", "text/event-stream; charset=utf-8")
	h.Set("Cache-Control", "no-cache")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, ": connected\n\n")
	fl.Flush()

	ch, unsub := s.hub.Subscribe()
	defer unsub()
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case c, ok := <-ch:
			if !ok {
				return
			}
			data, err := json.Marshal(c)
			if err != nil {
				continue
			}
			if _, err := fmt.Fprintf(w, "event: change\ndata: %s\n\n", data); err != nil {
				return
			}
			fl.Flush()
		case <-ticker.C:
			if _, err := io.WriteString(w, ": ping\n\n"); err != nil {
				return
			}
			fl.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

// --- accounts ---

type createAccountRequest struct {
	Remark       string `json:"remark"`
	Number       string `json:"number"`
	Display      string `json:"display"`
	BoundPhoneID int64  `json:"bound_phone_id"`
}

func (s *Server) handleCreateAccount(w http.ResponseWriter, r *http.Request) {
	var req createAccountRequest
	if err := decodeJSON(w, r, maxBody, &req); err != nil {
		writeBadRequest(w, err)
		return
	}
	if err := s.svc.ImportAccount(r.Context(), req.Remark, req.Number, req.Display, req.BoundPhoneID); err != nil {
		writeServiceError(w, err)
		return
	}
	id := int64(0)
	if acc, ok, err := s.svc.Repo().FindAccountByNumber(r.Context(), strings.TrimSpace(req.Number)); err == nil && ok {
		id = acc.ID
	}
	writeJSON(w, http.StatusCreated, map[string]int64{"id": id})
}

type updateAccountRequest struct {
	Remark       string `json:"remark"`
	Display      string `json:"display"`
	BoundPhoneID int64  `json:"bound_phone_id"`
}

func (s *Server) handleUpdateAccount(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req updateAccountRequest
	if err := decodeJSON(w, r, maxBody, &req); err != nil {
		writeBadRequest(w, err)
		return
	}
	if err := s.svc.UpdateAccount(r.Context(), id, req.Remark, req.Display, req.BoundPhoneID); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDeleteAccount(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := s.svc.DeleteAccount(r.Context(), id); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleBatchDelete(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs []int64 `json:"ids"`
	}
	if err := decodeJSON(w, r, maxBody, &req); err != nil {
		writeBadRequest(w, err)
		return
	}
	errs := []string{}
	for _, id := range req.IDs {
		if err := s.svc.DeleteAccount(r.Context(), id); err != nil {
			errs = append(errs, fmt.Sprintf("账号 %d: %v", id, err))
		}
	}
	writeJSON(w, http.StatusOK, map[string][]string{"errors": errs})
}

func (s *Server) handleImportCSV(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Text string `json:"text"`
	}
	if err := decodeJSON(w, r, maxCSVInput, &req); err != nil {
		writeBadRequest(w, err)
		return
	}
	res, err := s.svc.ImportCSV(r.Context(), req.Text)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleExportCSV(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs []int64 `json:"ids"`
	}
	if err := decodeJSON(w, r, maxBody, &req); err != nil {
		writeBadRequest(w, err)
		return
	}
	data, count, err := s.svc.AccountsCSV(r.Context(), req.IDs)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	h := w.Header()
	h.Set("Content-Type", "text/csv; charset=utf-8")
	h.Set("X-Export-Count", strconv.Itoa(count))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (s *Server) handleStartLogin(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var opts model.LoginOptions
	if err := decodeJSON(w, r, maxBody, &opts); err != nil {
		writeBadRequest(w, err)
		return
	}
	if err := s.svc.StartLogin(r.Context(), id, opts); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) handleConfirmLogin(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req struct {
		OK bool `json:"ok"`
	}
	if err := decodeJSON(w, r, maxBody, &req); err != nil {
		writeBadRequest(w, err)
		return
	}
	if _, err := s.svc.ConfirmLogin(r.Context(), id, req.OK); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleScreenshot(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	data, err := s.svc.Screenshot(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	h := w.Header()
	h.Set("Content-Type", "image/png")
	h.Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// --- phones, users, wallet, settings ---

func (s *Server) handlePhoneState(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req struct {
		Action string `json:"action"`
	}
	if err := decodeJSON(w, r, maxBody, &req); err != nil {
		writeBadRequest(w, err)
		return
	}
	if err := s.svc.SetPhoneState(r.Context(), id, req.Action); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleSyncPhones(w http.ResponseWriter, r *http.Request) {
	res, err := s.svc.SyncPhones(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleUserEnabled(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := decodeJSON(w, r, maxBody, &req); err != nil {
		writeBadRequest(w, err)
		return
	}
	if err := s.svc.SetUserEnabled(r.Context(), id, req.Enabled); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleTopUp(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Amount float64 `json:"amount"`
	}
	if err := decodeJSON(w, r, maxBody, &req); err != nil {
		writeBadRequest(w, err)
		return
	}
	if err := s.svc.TopUp(r.Context(), req.Amount); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleCharge(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Amount float64 `json:"amount"`
		Reason string  `json:"reason"`
	}
	if err := decodeJSON(w, r, maxBody, &req); err != nil {
		writeBadRequest(w, err)
		return
	}
	if err := s.svc.Charge(r.Context(), req.Amount, req.Reason); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	st, err := s.svc.Settings(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleSaveSettings(w http.ResponseWriter, r *http.Request) {
	var st model.Settings
	if err := decodeJSON(w, r, maxBody, &st); err != nil {
		writeBadRequest(w, err)
		return
	}
	if err := s.svc.SaveSettings(r.Context(), st); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- helpers ---

func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "bad_id", "无效的 ID")
		return 0, false
	}
	return id, true
}

func decodeJSON(w http.ResponseWriter, r *http.Request, limit int64, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(v); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return fmt.Errorf("请求内容不是有效的 JSON: %w", err)
	}
	return nil
}

func writeBadRequest(w http.ResponseWriter, err error) {
	writeError(w, http.StatusBadRequest, "bad_request", err.Error())
}

func writeServiceError(w http.ResponseWriter, err error) {
	status, code := statusFor(err)
	if status >= 500 {
		log.Printf("api error: %v", err)
	}
	writeError(w, status, code, err.Error())
}

func statusFor(err error) (int, string) {
	switch {
	case errors.Is(err, service.ErrNotFound):
		return http.StatusNotFound, "not_found"
	case errors.Is(err, service.ErrDuplicate):
		return http.StatusConflict, "duplicate"
	case errors.Is(err, service.ErrPhoneBusy):
		return http.StatusConflict, "phone_busy"
	case errors.Is(err, service.ErrNoPhone):
		return http.StatusConflict, "no_phone"
	case errors.Is(err, service.ErrLoginInProgress):
		return http.StatusConflict, "login_in_progress"
	case errors.Is(err, service.ErrAwaitingConfirm):
		return http.StatusConflict, "awaiting_confirm"
	case errors.Is(err, service.ErrNoActiveLogin):
		return http.StatusConflict, "no_active_login"
	case errors.Is(err, service.ErrLoginNotReady):
		return http.StatusConflict, "login_not_ready"
	case errors.Is(err, service.ErrInsufficientFunds):
		return http.StatusConflict, "insufficient_funds"
	case errors.Is(err, service.ErrInvalidLoginMethod),
		errors.Is(err, service.ErrInvalidAmount),
		errors.Is(err, service.ErrInvalidSettings),
		errors.Is(err, service.ErrEmptyNumber):
		return http.StatusBadRequest, "bad_request"
	case errors.Is(err, service.ErrClosed):
		return http.StatusServiceUnavailable, "closed"
	case errors.Is(err, driver.ErrProvider):
		return http.StatusBadGateway, "provider"
	default:
		return http.StatusInternalServerError, "internal"
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]string{"error": msg, "code": code})
}
