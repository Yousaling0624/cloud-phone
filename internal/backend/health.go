package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// HealthResult is what /api/health reports.
type HealthResult struct {
	OK       bool   `json:"ok"`
	Version  string `json:"version"`
	Database string `json:"database"`
}

// Health checks a server without a Backend around it, for the connection
// settings dialog (which must also work while the app has no backend).
func Health(ctx context.Context, baseURL, token string) (HealthResult, error) {
	base, err := normalizeBaseURL(baseURL)
	if err != nil {
		return HealthResult{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/health", nil)
	if err != nil {
		return HealthResult{}, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return HealthResult{}, fmt.Errorf("连接服务端失败: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
	if resp.StatusCode == http.StatusUnauthorized {
		return HealthResult{}, fmt.Errorf("认证失败：请检查访问令牌")
	}
	if resp.StatusCode != http.StatusOK {
		return HealthResult{}, fmt.Errorf("服务端返回 %s: %s", resp.Status, snippet(body))
	}
	var out HealthResult
	if err := json.Unmarshal(body, &out); err != nil {
		return HealthResult{}, fmt.Errorf("服务端响应不是有效的 JSON: %w", err)
	}
	if !out.OK {
		return HealthResult{}, fmt.Errorf("服务端未就绪（数据库: %s）", out.Database)
	}
	return out, nil
}

func normalizeBaseURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("请填写服务端地址")
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", fmt.Errorf("服务端地址必须是 http:// 或 https:// 开头的完整 URL")
	}
	return strings.TrimRight(raw, "/"), nil
}

func snippet(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 200 {
		s = s[:200] + "…"
	}
	return s
}
