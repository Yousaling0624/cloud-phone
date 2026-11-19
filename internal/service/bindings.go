package service

import (
	"context"
	"fmt"
	"strings"

	"cloud-phone/internal/model"
)

// bindingTarget is the validated phone an account should point at; the zero
// value means "no phone".
type bindingTarget struct {
	phoneID   int64
	phoneName string
}

// UpdateAccount saves the editable profile fields and the phone binding.
// The phone is checked before anything is written, and the store applies the
// binding transactionally.
func (s *Service) UpdateAccount(ctx context.Context, id int64, remark, display string, phoneID int64) error {
	s.bindMu.Lock()
	if _, err := s.repo.GetAccount(ctx, id); err != nil {
		s.bindMu.Unlock()
		return wrapNotFound("账号", id, err)
	}
	target, err := s.resolveBindingLocked(ctx, id, phoneID)
	if err != nil {
		s.bindMu.Unlock()
		return err
	}
	if err := s.repo.UpdateAccountProfile(ctx, id, strings.TrimSpace(remark), strings.TrimSpace(display)); err != nil {
		s.bindMu.Unlock()
		return err
	}
	if err := s.repo.SetAccountBinding(ctx, id, target.phoneID, target.phoneName); err != nil {
		s.bindMu.Unlock()
		return err
	}
	s.bindMu.Unlock()

	s.notify(model.Change{Kind: "accounts", AccountID: id})
	s.notify(model.Change{Kind: "phones"})
	return nil
}

// SetAccountPhone binds one account to one phone (0 unbinds). It exists for
// callers that only change the binding; the account form uses UpdateAccount.
func (s *Service) SetAccountPhone(ctx context.Context, accountID, phoneID int64) error {
	s.bindMu.Lock()
	if _, err := s.repo.GetAccount(ctx, accountID); err != nil {
		s.bindMu.Unlock()
		return wrapNotFound("账号", accountID, err)
	}
	target, err := s.resolveBindingLocked(ctx, accountID, phoneID)
	if err != nil {
		s.bindMu.Unlock()
		return err
	}
	if err := s.repo.SetAccountBinding(ctx, accountID, target.phoneID, target.phoneName); err != nil {
		s.bindMu.Unlock()
		return err
	}
	s.bindMu.Unlock()

	s.notify(model.Change{Kind: "accounts", AccountID: accountID})
	s.notify(model.Change{Kind: "phones"})
	return nil
}

// resolveBindingLocked validates the target while the caller holds bindMu.
// A phone already bound to a different account is refused; binding it to the
// same account again is fine.
func (s *Service) resolveBindingLocked(ctx context.Context, accountID, phoneID int64) (bindingTarget, error) {
	if phoneID == 0 {
		return bindingTarget{}, nil
	}
	p, err := s.repo.GetPhone(ctx, phoneID)
	if err != nil {
		return bindingTarget{}, wrapNotFound("云机", phoneID, err)
	}
	if p.AccountID != 0 && p.AccountID != accountID {
		return bindingTarget{}, fmt.Errorf("云机 %s: %w", p.Name, ErrPhoneBusy)
	}
	return bindingTarget{phoneID: p.ID, phoneName: p.Name}, nil
}

// RepairBindings makes the three copies of a binding agree at startup:
// phones.account_id is authoritative, accounts.bound_phone_id is the reverse
// lookup key, and accounts.bound_phone is only the display name. It returns
// how many rows it rewrote.
func (s *Service) RepairBindings(ctx context.Context) (int, error) {
	s.bindMu.Lock()
	defer s.bindMu.Unlock()

	accounts, err := s.repo.ListAccounts(ctx)
	if err != nil {
		return 0, err
	}
	phones, err := s.repo.ListPhones(ctx)
	if err != nil {
		return 0, err
	}
	accByID := make(map[int64]model.Account, len(accounts))
	for _, a := range accounts {
		accByID[a.ID] = a
	}
	phoneByID := make(map[int64]model.CloudPhone, len(phones))
	for _, p := range phones {
		phoneByID[p.ID] = p
	}

	fixed := 0
	// A phone pointing at a deleted account is cleared first.
	for _, p := range phones {
		if p.AccountID == 0 {
			continue
		}
		if _, ok := accByID[p.AccountID]; ok {
			continue
		}
		if err := s.repo.SetPhoneAccount(ctx, p.ID, 0); err != nil {
			return fixed, err
		}
		p.AccountID = 0
		phoneByID[p.ID] = p
		fixed++
	}

	// Decide one phone per account. phones.account_id is authoritative; when
	// several phones claim the same account, the account's bound_phone_id
	// wins, otherwise the smallest phone id does. Phones are ordered by id.
	desired := make(map[int64]int64, len(accounts))
	claimed := make(map[int64]bool, len(phones))
	for _, p := range phones {
		if p.AccountID == 0 {
			continue
		}
		acc := accByID[p.AccountID]
		cur, ok := desired[acc.ID]
		if !ok || acc.BoundPhoneID == p.ID {
			if ok && cur != p.ID {
				delete(claimed, cur)
			}
			desired[acc.ID] = p.ID
			claimed[p.ID] = true
		}
	}

	// Adopt a still-unclaimed phone for a legacy account, preferring the
	// explicit id and falling back to a unique name match.
	for _, a := range accounts {
		if _, ok := desired[a.ID]; ok {
			continue
		}
		if a.BoundPhoneID != 0 {
			p, ok := phoneByID[a.BoundPhoneID]
			if ok && p.AccountID == 0 && !claimed[p.ID] {
				desired[a.ID] = p.ID
				claimed[p.ID] = true
				continue
			}
		}
		if a.BoundPhoneID != 0 || a.BoundPhone == "" {
			continue
		}
		var match model.CloudPhone
		matches := 0
		for _, p := range phones {
			if p.Name == a.BoundPhone && p.AccountID == 0 && !claimed[p.ID] {
				match = p
				matches++
			}
		}
		if matches == 1 {
			desired[a.ID] = match.ID
			claimed[match.ID] = true
		}
	}

	for _, a := range accounts {
		wantID := desired[a.ID]
		wantName := ""
		if wantID != 0 {
			wantName = phoneByID[wantID].Name
		}
		if a.BoundPhoneID == wantID && a.BoundPhone == wantName {
			if wantID != 0 && phoneByID[wantID].AccountID == a.ID {
				continue
			}
			if wantID == 0 && !phoneClaims(phones, a.ID) {
				continue
			}
		}
		if err := s.repo.SetAccountBinding(ctx, a.ID, wantID, wantName); err != nil {
			return fixed, err
		}
		fixed++
	}
	return fixed, nil
}

func phoneClaims(phones []model.CloudPhone, accountID int64) bool {
	for _, p := range phones {
		if p.AccountID == accountID {
			return true
		}
	}
	return false
}
