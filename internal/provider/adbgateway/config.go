// Package adbgateway exposes physical Android devices over the platform's
// cloud-phone provider protocol. It drives the devices with the adb binary;
// screen control is left to a separate scrcpy/Escrcpy session operated by a
// human.
package adbgateway

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Config is the optional device list. Without it the gateway discovers every
// device that adb reports.
type Config struct {
	Devices []DeviceConfig `json:"devices"`
}

// DeviceConfig pins a device's external id and display metadata. Set Serial
// for a USB device or Address for a WiFi ADB endpoint. Address wins when both
// are set because it is what adb addresses.
type DeviceConfig struct {
	Serial  string `json:"serial"`
	Address string `json:"address"`
	Name    string `json:"name"`
	Region  string `json:"region"`
	Image   string `json:"image"`
}

// ExternalID is the stable identity stored as CloudPhone.ExternalID. A USB
// serial wins over the TCP address so moving a device to wireless ADB does
// not duplicate the phone or break its account binding.
func (d DeviceConfig) ExternalID() string {
	if s := strings.TrimSpace(d.Serial); s != "" {
		return s
	}
	return strings.TrimSpace(d.Address)
}

// Selector is what adb's -s flag receives. A configured TCP address wins
// because that is how a wireless device is reached.
func (d DeviceConfig) Selector() string {
	if s := strings.TrimSpace(d.Address); s != "" {
		return s
	}
	return strings.TrimSpace(d.Serial)
}

// LoadConfig reads and validates a config file. An empty path yields an
// empty config, which means adb auto-discovery.
func LoadConfig(path string) (Config, error) {
	if strings.TrimSpace(path) == "" {
		return Config{}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("读取设备配置: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("解析设备配置 %s: %w", path, err)
	}
	seenIDs := make(map[string]bool, len(cfg.Devices))
	seenSelectors := make(map[string]bool, len(cfg.Devices))
	for i := range cfg.Devices {
		d := &cfg.Devices[i]
		d.Serial = strings.TrimSpace(d.Serial)
		d.Address = strings.TrimSpace(d.Address)
		d.Name = strings.TrimSpace(d.Name)
		d.Region = strings.TrimSpace(d.Region)
		d.Image = strings.TrimSpace(d.Image)
		id := d.ExternalID()
		if id == "" {
			return Config{}, fmt.Errorf("设备配置第 %d 项缺少 serial 或 address", i+1)
		}
		if !safeSelector(d.Serial) || !safeSelector(d.Address) {
			return Config{}, fmt.Errorf("设备配置第 %d 项的 serial/address 含非法字符", i+1)
		}
		selector := d.Selector()
		if seenIDs[id] {
			return Config{}, fmt.Errorf("设备配置重复: %s", id)
		}
		if seenSelectors[selector] {
			return Config{}, fmt.Errorf("设备地址重复: %s", selector)
		}
		seenIDs[id] = true
		seenSelectors[selector] = true
	}
	return cfg, nil
}

// safeSelector rejects values that could be read as an adb option or that
// contain characters adb cannot address. It deliberately allows colons and
// IPv6 brackets for TCP endpoints.
func safeSelector(s string) bool {
	if s == "" {
		return true
	}
	if strings.HasPrefix(s, "-") {
		return false
	}
	for _, r := range s {
		if r <= ' ' || r == 0x7f {
			return false
		}
	}
	return true
}
