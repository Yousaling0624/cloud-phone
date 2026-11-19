package service

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"cloud-phone/internal/model"
)

// Settings returns the shared settings, with defaults filled in.
func (s *Service) Settings(ctx context.Context) (model.Settings, error) {
	return s.repo.GetSettings(ctx)
}

// SaveSettings validates and persists the shared settings.
func (s *Service) SaveSettings(ctx context.Context, st model.Settings) error {
	st = st.Normalized()
	if st.ProviderBaseURL != "" {
		u, err := url.Parse(st.ProviderBaseURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return fmt.Errorf("%w：网关地址必须是 http:// 或 https:// 开头的完整 URL", ErrInvalidSettings)
		}
		st.ProviderBaseURL = strings.TrimRight(st.ProviderBaseURL, "/")
	}
	if err := s.repo.SaveSettings(ctx, st); err != nil {
		return err
	}
	s.notify(model.Change{Kind: "settings"})
	return nil
}
