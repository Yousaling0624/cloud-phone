package driver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"cloud-phone/internal/model"
)

// Gateway implements the three drivers over the provider HTTP protocol. The
// base URL and token come from the shared settings on every call, so a
// provider change applies without restarting; when no provider is configured
// the built-in demo gateway is used.
type Gateway struct {
	Settings    func(context.Context) (model.Settings, error)
	DemoBaseURL string
	DemoToken   string
	Client      *http.Client
}

// NewGateway builds a gateway over a settings reader and a demo fallback.
func NewGateway(settings func(context.Context) (model.Settings, error), demoBaseURL, demoToken string) *Gateway {
	return &Gateway{Settings: settings, DemoBaseURL: demoBaseURL, DemoToken: demoToken}
}

func (g *Gateway) endpoint(ctx context.Context) (string, string, error) {
	if g.Settings == nil {
		return "", "", fmt.Errorf("%w: 未配置设置读取器", ErrProvider)
	}
	st, err := g.Settings(ctx)
	if err != nil {
		return "", "", fmt.Errorf("%w: 读取网关设置: %v", ErrProvider, err)
	}
	if st.ProviderBaseURL != "" {
		return strings.TrimRight(st.ProviderBaseURL, "/"), st.ProviderToken, nil
	}
	if g.DemoBaseURL != "" {
		return strings.TrimRight(g.DemoBaseURL, "/"), g.DemoToken, nil
	}
	return "", "", fmt.Errorf("%w: 未配置云机网关地址", ErrProvider)
}

// --- PhoneDriver ---

func (g *Gateway) Start(ctx context.Context, phone model.CloudPhone) error {
	return g.power(ctx, phone, "start")
}

func (g *Gateway) Stop(ctx context.Context, phone model.CloudPhone) error {
	return g.power(ctx, phone, "stop")
}

func (g *Gateway) Restart(ctx context.Context, phone model.CloudPhone) error {
	return g.power(ctx, phone, "restart")
}

func (g *Gateway) power(ctx context.Context, phone model.CloudPhone, action string) error {
	device, err := deviceID(phone)
	if err != nil {
		return err
	}
	callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	resp, err := g.do(callCtx, http.MethodPost,
		"/v1/phones/"+url.PathEscape(device)+"/power",
		map[string]string{"action": action})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	return nil
}

func (g *Gateway) Screenshot(ctx context.Context, phone model.CloudPhone) ([]byte, error) {
	device, err := deviceID(phone)
	if err != nil {
		return nil, err
	}
	callCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	resp, err := g.do(callCtx, http.MethodGet,
		"/v1/phones/"+url.PathEscape(device)+"/screenshot", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("%w: 读取截图: %v", ErrProvider, err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("%w: 网关返回了空截图", ErrProvider)
	}
	return data, nil
}

// --- SmsDriver ---

func (g *Gateway) WaitCode(ctx context.Context, phone string) (string, error) {
	q := url.Values{}
	q.Set("phone", phone)
	resp, err := g.do(ctx, http.MethodGet, "/v1/sms/wait?"+q.Encode(), nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var out struct {
		Code string `json:"code"`
	}
	if err := decodeProvider(resp, &out); err != nil {
		return "", err
	}
	if out.Code == "" {
		return "", fmt.Errorf("%w: 网关没有返回验证码", ErrProvider)
	}
	return out.Code, nil
}

// --- LoginDriver ---

type loginBody struct {
	Rule        string             `json:"rule"`
	AccountID   int64              `json:"account_id"`
	Number      string             `json:"number"`
	Display     string             `json:"display"`
	VerifyPhone string             `json:"verify_phone"`
	Phone       string             `json:"phone"`
	Options     model.LoginOptions `json:"options"`
}

func (g *Gateway) Execute(ctx context.Context, req LoginRequest) (LoginResult, error) {
	phone := ""
	if req.Phone.ExternalID != "" {
		phone = req.Phone.ExternalID
	} else if req.Phone.Name != "" {
		phone = req.Phone.Name
	} else {
		phone = req.Account.BoundPhone
	}
	body := loginBody{
		Rule: req.Rule, AccountID: req.Account.ID, Number: req.Account.Number,
		Display: req.Account.Display, VerifyPhone: req.Account.VerifyPhone,
		Phone: phone, Options: req.Options,
	}
	resp, err := g.do(ctx, http.MethodPost, "/v1/login/execute", body)
	if err != nil {
		return LoginResult{}, err
	}
	defer resp.Body.Close()
	var out struct {
		OK      bool   `json:"ok"`
		Code    string `json:"code"`
		Message string `json:"message"`
		Manual  bool   `json:"manual"`
	}
	if err := decodeProvider(resp, &out); err != nil {
		return LoginResult{}, err
	}
	return LoginResult{OK: out.OK, Code: out.Code, Message: out.Message, Manual: out.Manual}, nil
}

// --- DeviceLister ---

// ListDevices asks the gateway which phones it can address. A provider that
// does not implement GET /v1/phones returns a normal gateway error.
func (g *Gateway) ListDevices(ctx context.Context) ([]DeviceInfo, error) {
	callCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	resp, err := g.do(callCtx, http.MethodGet, "/v1/phones", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out struct {
		Phones []DeviceInfo `json:"phones"`
	}
	if err := decodeProvider(resp, &out); err != nil {
		return nil, err
	}
	for _, d := range out.Phones {
		if strings.TrimSpace(d.ExternalID) == "" {
			return nil, fmt.Errorf("%w: 网关返回了没有设备号的云机", ErrProvider)
		}
	}
	return out.Phones, nil
}

// --- plumbing ---

func deviceID(phone model.CloudPhone) (string, error) {
	if phone.ExternalID != "" {
		return phone.ExternalID, nil
	}
	if phone.Name != "" {
		return phone.Name, nil
	}
	return "", fmt.Errorf("%w: 云机没有设备号", ErrProvider)
}

// do sends the request and returns a non-2xx response as an error. The
// caller owns the body.
func (g *Gateway) do(ctx context.Context, method, path string, in any) (*http.Response, error) {
	base, token, err := g.endpoint(ctx)
	if err != nil {
		return nil, err
	}
	var body io.Reader
	if in != nil {
		data, err := json.Marshal(in)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, base+path, body)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrProvider, err)
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	client := g.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("%w: %v", ErrProvider, ctx.Err())
		}
		return nil, fmt.Errorf("%w: %v", ErrProvider, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		resp.Body.Close()
		msg := providerErrorMessage(data)
		if msg == "" {
			msg = resp.Status
		} else {
			msg = resp.Status + ": " + msg
		}
		return nil, fmt.Errorf("%w: %s", ErrProvider, msg)
	}
	return resp, nil
}

func decodeProvider(resp *http.Response, out any) error {
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("%w: 读取响应: %v", ErrProvider, err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("%w: 响应不是有效的 JSON: %v", ErrProvider, err)
	}
	return nil
}

func providerErrorMessage(data []byte) string {
	var e struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if json.Unmarshal(data, &e) == nil {
		if e.Error != "" {
			return e.Error
		}
		if e.Message != "" {
			return e.Message
		}
	}
	s := strings.TrimSpace(string(data))
	if len(s) > 200 {
		s = s[:200] + "…"
	}
	return s
}

var _ PhoneDriver = (*Gateway)(nil)
var _ SmsDriver = (*Gateway)(nil)
var _ LoginDriver = (*Gateway)(nil)
var _ DeviceLister = (*Gateway)(nil)
