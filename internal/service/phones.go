package service

import (
	"context"
	"fmt"
	"strings"

	"cloud-phone/internal/driver"
	"cloud-phone/internal/model"
)

// SyncPhones asks the provider for its current device list and creates or
// refreshes the platform's phone records by external id. Bindings are kept:
// only provider-owned display fields and state are updated. Devices missing
// from the response are not deleted, so a temporarily offline gateway cannot
// wipe the operator's inventory.
func (s *Service) SyncPhones(ctx context.Context) (model.PhoneSyncResult, error) {
	lister, ok := s.phone.(driver.DeviceLister)
	if !ok {
		return model.PhoneSyncResult{}, fmt.Errorf("%w: 当前云机驱动不支持设备同步", driver.ErrProvider)
	}
	devices, err := lister.ListDevices(ctx)
	if err != nil {
		return model.PhoneSyncResult{}, err
	}

	s.phoneSyncMu.Lock()
	defer s.phoneSyncMu.Unlock()

	res := model.PhoneSyncResult{Total: len(devices)}
	for _, d := range devices {
		externalID := strings.TrimSpace(d.ExternalID)
		if externalID == "" {
			continue
		}
		name := strings.TrimSpace(d.Name)
		if name == "" {
			name = externalID
		}
		region := strings.TrimSpace(d.Region)
		if region == "" {
			region = "本地"
		}
		image := strings.TrimSpace(d.Image)
		if image == "" {
			image = "Android"
		}
		ip := strings.TrimSpace(d.IP)

		existing, found, err := s.repo.FindPhoneByExternalID(ctx, externalID)
		if err != nil {
			return res, err
		}
		state := reconcilePhoneState(d, existing, found)
		if !found {
			p := model.CloudPhone{
				Name: name, ExternalID: externalID, Region: region,
				Image: image, State: state, IP: ip,
			}
			if err := s.repo.CreatePhone(ctx, &p); err != nil {
				return res, err
			}
			res.Added++
			continue
		}
		if err := s.repo.UpdatePhoneInfo(ctx, existing.ID, name, region, image, state, ip); err != nil {
			return res, err
		}
		res.Updated++
	}
	if res.Added > 0 || res.Updated > 0 {
		s.notify(model.Change{Kind: "phones"})
	}
	return res, nil
}

// reconcilePhoneState maps provider connectivity onto the platform's phone
// state. A sync must not resurrect a phone the operator deliberately stopped
// or restarted, so only error/idle rows are corrected when the device is
// reachable; an unreachable device is always shown as an error.
func reconcilePhoneState(d driver.DeviceInfo, existing model.CloudPhone, found bool) model.PhoneState {
	if d.State != driver.DeviceOnline {
		return model.PhoneError
	}
	if !found || existing.State == model.PhoneError || existing.State == "" {
		// A reachable physical device is powered on even before it is
		// allocated; "idle" is the platform's unallocated cloud-phone state,
		// not an adb connectivity state.
		return model.PhoneRunning
	}
	if existing.AccountID != 0 && existing.State == model.PhoneIdle {
		return model.PhoneRunning
	}
	return existing.State
}
