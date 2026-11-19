package service

import (
	"context"
	"time"

	"cloud-phone/internal/model"
)

// Snapshot reads everything the console renders in one call. Empty lists are
// returned as [] so the page can map over them without checks.
func (s *Service) Snapshot(ctx context.Context) (model.Snapshot, error) {
	repo := s.repo
	var out model.Snapshot
	var err error
	if out.Accounts, err = repo.ListAccounts(ctx); err != nil {
		return out, err
	}
	if out.Phones, err = repo.ListPhones(ctx); err != nil {
		return out, err
	}
	if out.Sms, err = repo.ListSms(ctx); err != nil {
		return out, err
	}
	if out.Sims, err = repo.ListSims(ctx); err != nil {
		return out, err
	}
	if out.Users, err = repo.ListUsers(ctx); err != nil {
		return out, err
	}
	if out.Wallet, err = repo.Wallet(ctx); err != nil {
		return out, err
	}
	if out.Ledger, err = repo.ListLedger(ctx, 50); err != nil {
		return out, err
	}
	if out.Settings, err = repo.GetSettings(ctx); err != nil {
		return out, err
	}
	out.Tasks = s.Tasks()
	out.Now = time.Now()

	out.Accounts = nonNil(out.Accounts)
	out.Phones = nonNil(out.Phones)
	out.Sms = nonNil(out.Sms)
	out.Sims = nonNil(out.Sims)
	out.Users = nonNil(out.Users)
	out.Ledger = nonNil(out.Ledger)
	out.Tasks = nonNil(out.Tasks)
	return out, nil
}

func nonNil[T any](v []T) []T {
	if v == nil {
		return []T{}
	}
	return v
}
