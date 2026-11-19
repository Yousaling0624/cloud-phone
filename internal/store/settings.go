package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"cloud-phone/internal/model"
)

// GetSettings reads the single settings row. A missing row, or missing
// fields in an older row, fall back to model.DefaultSettings.
func (s *SQLite) GetSettings(ctx context.Context) (model.Settings, error) {
	out := model.DefaultSettings()
	rows, err := s.db.Query(ctx, `SELECT json FROM settings WHERE id = 1`)
	if err != nil {
		return out, err
	}
	if len(rows.Values) == 0 {
		return out, nil
	}
	raw, _ := rows.Values[0][0].(string)
	if raw == "" {
		return out, nil
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return model.DefaultSettings(), fmt.Errorf("decode settings: %w", err)
	}
	return out.Normalized(), nil
}

// SaveSettings writes the whole settings object in one upsert.
func (s *SQLite) SaveSettings(ctx context.Context, st model.Settings) error {
	data, err := json.Marshal(st.Normalized())
	if err != nil {
		return err
	}
	_, err = s.db.Execute(ctx, `INSERT INTO settings (id, json, updated_at) VALUES (1, ?, ?)
		ON CONFLICT(id) DO UPDATE SET json = excluded.json, updated_at = excluded.updated_at`,
		string(data), time.Now().Format(time.RFC3339))
	return err
}
