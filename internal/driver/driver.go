// Package driver isolates every external integration the platform needs:
// talking to a cloud-phone provider (boot, stop, screenshot), receiving SMS
// verification codes, and running a platform's login automation ("login
// rule"). A real provider implements the same interfaces; the gateway
// implementation talks HTTP to a provider service.
package driver

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math/rand/v2"
	"time"

	"cloud-phone/internal/model"
)

// ErrProvider marks a failure talking to a cloud-phone provider, as opposed
// to an invalid request. The HTTP server maps it to 502.
var ErrProvider = errors.New("云机网关调用失败")

// PhoneDriver talks to a cloud-phone provider.
type PhoneDriver interface {
	Start(ctx context.Context, phone model.CloudPhone) error
	Stop(ctx context.Context, phone model.CloudPhone) error
	Restart(ctx context.Context, phone model.CloudPhone) error
	// Screenshot returns a PNG of the phone's current screen, for the
	// console preview.
	Screenshot(ctx context.Context, phone model.CloudPhone) ([]byte, error)
}

// SmsDriver receives verification codes. WaitCode blocks until a code for
// phone arrives, or the context is done.
type SmsDriver interface {
	WaitCode(ctx context.Context, phone string) (code string, err error)
}

// LoginRequest is everything a login rule needs to run.
type LoginRequest struct {
	Rule    string
	Account model.Account
	// Phone is the account's bound phone, if any. Its ExternalID is what a
	// provider gateway addresses.
	Phone   model.CloudPhone
	Options model.LoginOptions
}

// LoginResult is what running a login rule produced.
type LoginResult struct {
	OK      bool
	Code    string // the verification code, when the rule got that far
	Message string // why it failed, or a note on success
	// Manual means the provider did not automate the login. The platform
	// marks the account ready and the operator completes the login on the
	// device (for example through scrcpy), then confirms it in the console.
	Manual bool
}

// LoginDriver runs a platform's login automation.
type LoginDriver interface {
	Execute(ctx context.Context, req LoginRequest) (LoginResult, error)
}

// Provider device connectivity states used by DeviceInfo.
const (
	DeviceOnline       = "online"
	DeviceOffline      = "offline"
	DeviceUnauthorized = "unauthorized"
)

// DeviceInfo is one phone a provider can address. It is used to create or
// refresh the platform's phone records; ExternalID is the provider's id and
// is stable across syncs. State describes whether adb can reach the device,
// not whether its screen is on.
type DeviceInfo struct {
	ExternalID string `json:"external_id"`
	Name       string `json:"name"`
	Region     string `json:"region"`
	Image      string `json:"image"`
	State      string `json:"state"`
	IP         string `json:"ip"`
}

// DeviceLister is implemented by providers that can enumerate their phones.
// PhoneDriver stays unchanged so a provider only implements what it has.
type DeviceLister interface {
	ListDevices(ctx context.Context) ([]DeviceInfo, error)
}

// --- mock implementations ---

// MockPhone answers as an always-available provider would, after a short
// simulated boot.
type MockPhone struct{}

func (MockPhone) Start(ctx context.Context, phone model.CloudPhone) error {
	return sleep(ctx, 600*time.Millisecond)
}
func (MockPhone) Stop(ctx context.Context, phone model.CloudPhone) error {
	return sleep(ctx, 300*time.Millisecond)
}
func (MockPhone) Restart(ctx context.Context, phone model.CloudPhone) error {
	return sleep(ctx, 800*time.Millisecond)
}

// Screenshot returns a tiny transparent PNG placeholder; a real driver
// returns the phone's actual framebuffer.
func (MockPhone) Screenshot(ctx context.Context, phone model.CloudPhone) ([]byte, error) {
	if err := sleep(ctx, 200*time.Millisecond); err != nil {
		return nil, err
	}
	return placeholderPNG, nil
}

// MockSms delivers a random 6-digit code after 3–8 seconds, unless the
// context ends first (the console caps this at 10 minutes).
type MockSms struct{}

func (MockSms) WaitCode(ctx context.Context, phone string) (string, error) {
	d := time.Duration(3+rand.IntN(6)) * time.Second
	if err := sleep(ctx, d); err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", rand.IntN(1_000_000)), nil
}

// MockLogin succeeds, or fails with the rule error, and delegates the code
// wait to an SmsDriver so the console shows the code when it arrives.
type MockLogin struct {
	Sms SmsDriver
	// FailRate is the chance (0..1) a login fails with the rule error.
	FailRate float64
}

func (m MockLogin) Execute(ctx context.Context, req LoginRequest) (LoginResult, error) {
	if rand.Float64() < m.FailRate {
		return LoginResult{OK: false, Message: "执行规则失败: " + req.Rule}, nil
	}
	code, err := m.Sms.WaitCode(ctx, req.Account.VerifyPhone)
	if err != nil {
		return LoginResult{}, err
	}
	return LoginResult{OK: true, Code: code, Message: "登录成功"}, nil
}

func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// placeholderPNG is a small valid PNG used by the mock phone, so the console
// preview path is exercised without a provider.
var placeholderPNG = func() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 16, 28))
	for y := 0; y < 28; y++ {
		for x := 0; x < 16; x++ {
			c := color.RGBA{24, 36, 72, 255}
			if x > 2 && x < 13 && y > 12 && y < 25 {
				c = color.RGBA{90, 140, 220, 255}
			}
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil
	}
	return buf.Bytes()
}()
