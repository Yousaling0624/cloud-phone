package backend

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"cloud-phone/internal/model"
)

const (
	callTimeout  = 30 * time.Second
	maxJSONBody  = 16 << 20
	maxCSVBody   = 8 << 20
	maxImageBody = 8 << 20
)

// Remote is the Backend that talks HTTP to a standalone server.
type Remote struct {
	baseURL string
	token   string
	client  *http.Client
}

// NewRemote builds a client for baseURL. The URL must be absolute.
func NewRemote(baseURL, token string) (*Remote, error) {
	base, err := normalizeBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	return &Remote{
		baseURL: base,
		token:   strings.TrimSpace(token),
		client:  &http.Client{},
	}, nil
}

func (r *Remote) Snapshot(ctx context.Context) (model.Snapshot, error) {
	var out model.Snapshot
	err := r.doJSON(ctx, http.MethodGet, "/api/snapshot", nil, &out)
	return out, err
}

type importAccountBody struct {
	Remark       string `json:"remark"`
	Number       string `json:"number"`
	Display      string `json:"display"`
	BoundPhoneID int64  `json:"bound_phone_id"`
}

func (r *Remote) ImportAccount(ctx context.Context, remark, number, display string, phoneID int64) error {
	return r.doJSON(ctx, http.MethodPost, "/api/accounts",
		importAccountBody{Remark: remark, Number: number, Display: display, BoundPhoneID: phoneID}, nil)
}

type updateAccountBody struct {
	Remark       string `json:"remark"`
	Display      string `json:"display"`
	BoundPhoneID int64  `json:"bound_phone_id"`
}

func (r *Remote) UpdateAccount(ctx context.Context, id int64, remark, display string, phoneID int64) error {
	return r.doJSON(ctx, http.MethodPatch, fmt.Sprintf("/api/accounts/%d", id),
		updateAccountBody{Remark: remark, Display: display, BoundPhoneID: phoneID}, nil)
}

func (r *Remote) DeleteAccount(ctx context.Context, id int64) error {
	return r.doJSON(ctx, http.MethodDelete, fmt.Sprintf("/api/accounts/%d", id), nil, nil)
}

func (r *Remote) DeleteAccounts(ctx context.Context, ids []int64) error {
	var out struct {
		Errors []string `json:"errors"`
	}
	if err := r.doJSON(ctx, http.MethodPost, "/api/accounts/batch-delete",
		map[string][]int64{"ids": ids}, &out); err != nil {
		return err
	}
	if len(out.Errors) > 0 {
		return errors.New(strings.Join(out.Errors, "；"))
	}
	return nil
}

func (r *Remote) ImportCSV(ctx context.Context, text string) (model.ImportResult, error) {
	var out model.ImportResult
	err := r.doJSON(ctx, http.MethodPost, "/api/accounts/import",
		map[string]string{"text": text}, &out)
	if out.Errors == nil {
		out.Errors = []string{}
	}
	return out, err
}

func (r *Remote) AccountsCSV(ctx context.Context, ids []int64) ([]byte, int, error) {
	resp, err := r.send(ctx, http.MethodPost, "/api/accounts/export.csv", map[string][]int64{"ids": ids})
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	if err := checkStatus(resp); err != nil {
		return nil, 0, err
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxCSVBody))
	if err != nil {
		return nil, 0, fmt.Errorf("读取导出内容: %w", err)
	}
	count, _ := strconv.Atoi(resp.Header.Get("X-Export-Count"))
	return data, count, nil
}

func (r *Remote) StartLogin(ctx context.Context, id int64, opts model.LoginOptions) error {
	// The server owns the attempt; a page that closes right after clicking
	// must not cancel the request before the server has accepted it.
	callCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), callTimeout)
	defer cancel()
	return r.doJSON(callCtx, http.MethodPost, fmt.Sprintf("/api/accounts/%d/login", id), opts, nil)
}

func (r *Remote) ConfirmLogin(ctx context.Context, id int64, ok bool) error {
	return r.doJSON(ctx, http.MethodPost, fmt.Sprintf("/api/accounts/%d/login/confirm", id),
		map[string]bool{"ok": ok}, nil)
}

func (r *Remote) SetPhoneState(ctx context.Context, id int64, action string) error {
	return r.doJSON(ctx, http.MethodPost, fmt.Sprintf("/api/phones/%d/state", id),
		map[string]string{"action": action}, nil)
}

func (r *Remote) SyncPhones(ctx context.Context) (model.PhoneSyncResult, error) {
	var out model.PhoneSyncResult
	err := r.doJSON(ctx, http.MethodPost, "/api/phones/sync", nil, &out)
	return out, err
}

func (r *Remote) SetUserEnabled(ctx context.Context, id int64, enabled bool) error {
	return r.doJSON(ctx, http.MethodPost, fmt.Sprintf("/api/users/%d/enabled", id),
		map[string]bool{"enabled": enabled}, nil)
}

func (r *Remote) TopUp(ctx context.Context, amount float64) error {
	return r.doJSON(ctx, http.MethodPost, "/api/wallet/topup",
		map[string]float64{"amount": amount}, nil)
}

func (r *Remote) Charge(ctx context.Context, amount float64, reason string) error {
	return r.doJSON(ctx, http.MethodPost, "/api/wallet/charge",
		map[string]any{"amount": amount, "reason": reason}, nil)
}

func (r *Remote) Screenshot(ctx context.Context, accountID int64) ([]byte, error) {
	resp, err := r.send(ctx, http.MethodGet, fmt.Sprintf("/api/accounts/%d/screenshot", accountID), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := checkStatus(resp); err != nil {
		return nil, err
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxImageBody))
	if err != nil {
		return nil, fmt.Errorf("读取云机画面: %w", err)
	}
	return data, nil
}

func (r *Remote) Settings(ctx context.Context) (model.Settings, error) {
	var out model.Settings
	err := r.doJSON(ctx, http.MethodGet, "/api/settings", nil, &out)
	return out, err
}

func (r *Remote) SaveSettings(ctx context.Context, s model.Settings) error {
	return r.doJSON(ctx, http.MethodPut, "/api/settings", s, nil)
}

// Subscribe follows the server's SSE stream and calls onChange for every
// change. It reconnects until ctx is canceled, unless the server rejects
// the token.
func (r *Remote) Subscribe(ctx context.Context, onChange func(model.Change)) error {
	backoff := time.Second
	for {
		if err := ctx.Err(); err != nil {
			return nil
		}
		err := r.streamOnce(ctx, onChange)
		if ctx.Err() != nil {
			return nil
		}
		if errors.Is(err, errUnauthorized) {
			return err
		}
		// A reconnect may have missed events; tell the page to resync.
		onChange(model.Change{Kind: "sync"})
		t := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			t.Stop()
			return nil
		case <-t.C:
		}
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}

func (r *Remote) streamOnce(ctx context.Context, onChange func(model.Change)) error {
	req, err := r.newRequest(ctx, http.MethodGet, "/api/events", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/event-stream")
	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return errUnauthorized
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("事件流返回 %s: %s", resp.Status, readSnippet(resp.Body))
	}
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64<<10), 1<<20)
	var data []string
	dispatch := func() error {
		if len(data) == 0 {
			return nil
		}
		raw := strings.Join(data, "\n")
		data = data[:0]
		var c model.Change
		if err := json.Unmarshal([]byte(raw), &c); err != nil {
			return nil // ignore a malformed event, keep the stream alive
		}
		onChange(c)
		return nil
	}
	for scanner.Scan() {
		line := strings.TrimSuffix(scanner.Text(), "\r")
		switch {
		case line == "":
			if err := dispatch(); err != nil {
				return err
			}
		case strings.HasPrefix(line, "data:"):
			v := strings.TrimPrefix(line, "data:")
			v = strings.TrimPrefix(v, " ")
			data = append(data, v)
		}
	}
	if err := scanner.Err(); err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("事件流中断: %w", err)
	}
	return io.EOF
}

var errUnauthorized = errors.New("认证失败：请检查访问令牌")

func (r *Remote) doJSON(ctx context.Context, method, path string, in, out any) error {
	callCtx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	resp, err := r.send(callCtx, method, path, in)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if err := checkStatus(resp); err != nil {
		return err
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxJSONBody))
		return nil
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxJSONBody))
	if err != nil {
		return fmt.Errorf("读取响应: %w", err)
	}
	if len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("服务端响应不是有效的 JSON: %w", err)
	}
	return nil
}

func (r *Remote) send(ctx context.Context, method, path string, in any) (*http.Response, error) {
	var body io.Reader
	if in != nil {
		data, err := json.Marshal(in)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(data)
	}
	req, err := r.newRequest(ctx, method, path, body)
	if err != nil {
		return nil, err
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := r.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("连接服务端失败: %w", ctx.Err())
		}
		return nil, fmt.Errorf("连接服务端失败: %w", err)
	}
	return resp, nil
}

func (r *Remote) newRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, r.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	if r.token != "" {
		req.Header.Set("Authorization", "Bearer "+r.token)
	}
	req.Header.Set("Accept", "application/json")
	return req, nil
}

// apiError is the server's uniform error envelope.
type apiError struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

func checkStatus(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
	var e apiError
	if json.Unmarshal(data, &e) == nil && e.Error != "" {
		if resp.StatusCode == http.StatusUnauthorized {
			return errUnauthorized
		}
		return errors.New(e.Error)
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return errUnauthorized
	}
	msg := snippet(data)
	if msg == "" {
		msg = resp.Status
	}
	return fmt.Errorf("服务端返回 %s: %s", resp.Status, msg)
}

func readSnippet(r io.Reader) string {
	data, _ := io.ReadAll(io.LimitReader(r, 4<<10))
	return snippet(data)
}
