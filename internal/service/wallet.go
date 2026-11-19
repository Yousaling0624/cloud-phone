package service

import (
	"context"
	"fmt"
	"math"
	"strings"

	"cloud-phone/internal/model"
)

// Wallet returns the current balance and the newest ledger entries.
func (s *Service) Wallet(ctx context.Context) (model.Wallet, []model.LedgerEntry, error) {
	w, err := s.repo.Wallet(ctx)
	if err != nil {
		return model.Wallet{}, nil, err
	}
	ledger, err := s.repo.ListLedger(ctx, 50)
	if err != nil {
		return model.Wallet{}, nil, err
	}
	return w, ledger, nil
}

// TopUp adds money to the wallet. The read-modify-write is serialized so two
// concurrent top-ups cannot lose one another.
func (s *Service) TopUp(ctx context.Context, amount float64) error {
	amount = round2(amount)
	if err := validAmount(amount); err != nil {
		return err
	}
	s.walletMu.Lock()
	w, err := s.repo.Wallet(ctx)
	if err != nil {
		s.walletMu.Unlock()
		return err
	}
	newBalance := round2(w.Balance + amount)
	entry := model.LedgerEntry{Amount: amount, Reason: "充值"}
	if err := s.repo.ApplyWalletChange(ctx, newBalance, &entry); err != nil {
		s.walletMu.Unlock()
		return err
	}
	s.walletMu.Unlock()

	s.notify(model.Change{Kind: "wallet"})
	return nil
}

// Charge debits the wallet. A charge beyond the balance is refused before
// anything is written.
func (s *Service) Charge(ctx context.Context, amount float64, reason string) error {
	amount = round2(amount)
	if err := validAmount(amount); err != nil {
		return err
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "扣费"
	}
	s.walletMu.Lock()
	w, err := s.repo.Wallet(ctx)
	if err != nil {
		s.walletMu.Unlock()
		return err
	}
	if amount > w.Balance {
		s.walletMu.Unlock()
		return fmt.Errorf("%w（当前余额 %.2f）", ErrInsufficientFunds, w.Balance)
	}
	newBalance := round2(w.Balance - amount)
	entry := model.LedgerEntry{Amount: -amount, Reason: reason}
	if err := s.repo.ApplyWalletChange(ctx, newBalance, &entry); err != nil {
		s.walletMu.Unlock()
		return err
	}
	s.walletMu.Unlock()

	s.notify(model.Change{Kind: "wallet"})
	return nil
}

func validAmount(v float64) error {
	if math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 || v > 10_000_000 {
		return ErrInvalidAmount
	}
	return nil
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
