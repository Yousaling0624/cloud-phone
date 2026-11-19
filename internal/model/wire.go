package model

import (
	"strings"
	"time"
)

// Login method keys. The Chinese labels live in the frontend.
const (
	LoginMethodPassword = "password"
	LoginMethodSMS      = "sms"
	LoginMethodScan     = "scan"
)

// ValidLoginMethod reports whether method is one of the supported keys.
func ValidLoginMethod(method string) bool {
	switch method {
	case LoginMethodPassword, LoginMethodSMS, LoginMethodScan:
		return true
	}
	return false
}

// Change says what changed, so a client can refresh what it needs. The
// current frontend reloads its whole snapshot either way.
type Change struct {
	Kind      string `json:"kind"` // accounts, phones, sms, sims, users, wallet, tasks, settings, sync
	AccountID int64  `json:"account_id"`
}

// Snapshot is everything the console renders, read in one round trip.
type Snapshot struct {
	Accounts []Account     `json:"accounts"`
	Phones   []CloudPhone  `json:"phones"`
	Sms      []Sms         `json:"sms"`
	Sims     []SimSlot     `json:"sims"`
	Users    []User        `json:"users"`
	Wallet   Wallet        `json:"wallet"`
	Ledger   []LedgerEntry `json:"ledger"`
	Tasks    []Task        `json:"tasks"`
	Settings Settings      `json:"settings"`
	Now      time.Time     `json:"now"`
}

// ImportResult reports what a CSV import added and which lines failed.
type ImportResult struct {
	Added  int      `json:"added"`
	Errors []string `json:"errors"`
}

// PhoneSyncResult reports what a provider device sync changed.
type PhoneSyncResult struct {
	Added   int `json:"added"`
	Updated int `json:"updated"`
	Total   int `json:"total"`
}

// LoginOptions are the console's per-attempt choices.
type LoginOptions struct {
	Method   string `json:"method"` // password, sms, scan
	AutoFill bool   `json:"auto_fill"`
	AutoBind bool   `json:"auto_bind"`
	SaveLog  bool   `json:"save_log"`
}

// Normalized fills the default method and rejects unknown ones.
func (o LoginOptions) Normalized() (LoginOptions, bool) {
	o.Method = strings.TrimSpace(o.Method)
	if o.Method == "" {
		o.Method = LoginMethodPassword
	}
	return o, ValidLoginMethod(o.Method)
}

// Settings are the platform settings shared by every client of a server.
type Settings struct {
	ProviderBaseURL string `json:"provider_base_url"` // empty = built-in demo gateway
	ProviderToken   string `json:"provider_token"`
	LoginMethod     string `json:"login_method"`
	AutoFill        bool   `json:"auto_fill"`
	AutoBind        bool   `json:"auto_bind"`
	SaveLog         bool   `json:"save_log"`
}

// DefaultSettings are used when the database has no settings row yet.
func DefaultSettings() Settings {
	return Settings{
		LoginMethod: LoginMethodPassword,
		AutoFill:    true,
		AutoBind:    true,
		SaveLog:     false,
	}
}

// Normalized trims the URLs and fills defaults without losing explicit
// false values.
func (s Settings) Normalized() Settings {
	s.ProviderBaseURL = strings.TrimSpace(s.ProviderBaseURL)
	s.ProviderToken = strings.TrimSpace(s.ProviderToken)
	s.LoginMethod = strings.TrimSpace(s.LoginMethod)
	if s.LoginMethod == "" || !ValidLoginMethod(s.LoginMethod) {
		s.LoginMethod = LoginMethodPassword
	}
	return s
}
