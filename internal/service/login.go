package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"cloud-phone/internal/driver"
	"cloud-phone/internal/model"
)

const (
	loginTimeout = 10 * time.Minute
	defaultRule  = "qq_account_management_login_rule"
)

// StartLogin begins a login: it marks the account waiting, runs the login
// driver in the background, and reports progress through the notifier. The
// attempt is owned by the service, not by the calling request, so a page
// call returning does not cancel it.
func (s *Service) StartLogin(ctx context.Context, id int64, opts model.LoginOptions) error {
	opts, ok := opts.Normalized()
	if !ok {
		return ErrInvalidLoginMethod
	}

	s.mu.Lock()
	if s.closing {
		s.mu.Unlock()
		return ErrClosed
	}
	if _, exists := s.attempts[id]; exists {
		s.mu.Unlock()
		return ErrLoginInProgress
	}
	a, err := s.repo.GetAccount(ctx, id)
	if err != nil {
		s.mu.Unlock()
		return wrapNotFound("账号", id, err)
	}
	if a.Status == model.AccountLoggingIn || a.Login == model.LoginWaiting {
		s.mu.Unlock()
		return ErrLoginInProgress
	}
	if a.Login == model.LoginReady {
		s.mu.Unlock()
		return ErrAwaitingConfirm
	}
	rule := a.LoginRule
	if rule == "" {
		rule = defaultRule
	}
	if err := s.repo.UpdateAccountLogin(ctx, id, model.AccountLoggingIn, model.LoginWaiting, rule, "", a.VerifyPhone); err != nil {
		s.mu.Unlock()
		return err
	}

	runCtx, cancel := context.WithTimeout(s.rootCtx, loginTimeout)
	s.nextGen++
	attempt := &loginAttempt{gen: s.nextGen, cancel: cancel, opts: opts}
	task := s.addTaskLocked(model.Task{
		Kind: model.TaskLogin, AccountID: id,
		Label: fmt.Sprintf("账号 %s 登扣中", a.Number), StartedAt: time.Now(),
	})
	attempt.taskID = task.ID
	s.attempts[id] = attempt
	s.wg.Add(1)
	s.mu.Unlock()

	s.notify(model.Change{Kind: "accounts", AccountID: id})
	s.notify(model.Change{Kind: "tasks", AccountID: id})

	a.LoginRule = rule
	a.Status = model.AccountLoggingIn
	a.Login = model.LoginWaiting
	a.VerifyCode = ""
	phone, _ := s.phoneForAccount(ctx, a)

	go func() {
		defer s.wg.Done()
		s.runLogin(runCtx, attempt, a, phone, opts)
	}()
	return nil
}

// runLogin finishes one attempt. The generation check under s.mu is the
// guard that prevents a canceled or superseded worker from resurrecting an
// account after the user confirmed failure.
func (s *Service) runLogin(ctx context.Context, attempt *loginAttempt, a model.Account, phone model.CloudPhone, opts model.LoginOptions) {
	started := time.Now()
	res, err := s.login.Execute(ctx, driver.LoginRequest{
		Rule: a.LoginRule, Account: a, Phone: phone, Options: opts,
	})
	if opts.SaveLog {
		log.Printf("login rule=%q account=%s ok=%v code=%q manual=%v err=%v elapsed=%s",
			a.LoginRule, a.Number, res.OK, res.Code, res.Manual, err, time.Since(started).Round(time.Millisecond))
	}

	s.mu.Lock()
	if s.closing {
		s.finishAttemptLocked(a.ID, attempt)
		s.mu.Unlock()
		return
	}
	if s.attempts[a.ID] != attempt {
		// Canceled by ConfirmLogin, delete, or a newer attempt. The canceller
		// already cleaned up; never write state for a stale attempt.
		s.mu.Unlock()
		return
	}

	status := model.AccountAbnormal
	login := model.LoginFail
	code := ""
	if err == nil && res.OK && (res.Code != "" || res.Manual) {
		status = model.AccountLoggingIn
		login = model.LoginReady
		code = res.Code
	}
	// The driver context may be canceled (timeout), so final writes use a
	// context that is not canceled by it.
	writeCtx := context.WithoutCancel(ctx)
	perr := s.repo.UpdateAccountLogin(writeCtx, a.ID, status, login, a.LoginRule, code, a.VerifyPhone)
	s.finishAttemptLocked(a.ID, attempt)
	attempt.cancel()
	s.mu.Unlock()

	s.notify(model.Change{Kind: "accounts", AccountID: a.ID})
	s.notify(model.Change{Kind: "tasks", AccountID: a.ID})
	if perr != nil {
		log.Printf("persist login result for account %d: %v", a.ID, perr)
		return
	}
	if login == model.LoginReady && code != "" {
		_ = s.repo.AddSms(writeCtx, &model.Sms{
			Phone: a.VerifyPhone, Code: code, Content: "登录验证码：" + code,
		})
		s.notify(model.Change{Kind: "sms", AccountID: a.ID})
	}
}

// ConfirmLogin records the user's verdict. A failure cancels the running
// provider work and clears the code, so the failed attempt cannot come back
// and a stale code cannot mask the failure card.
func (s *Service) ConfirmLogin(ctx context.Context, id int64, ok bool) (model.Account, error) {
	s.mu.Lock()
	a, err := s.repo.GetAccount(ctx, id)
	if err != nil {
		s.mu.Unlock()
		return model.Account{}, wrapNotFound("账号", id, err)
	}
	if ok && a.Login != model.LoginReady {
		s.mu.Unlock()
		return a, ErrLoginNotReady
	}
	if !ok && a.Login != model.LoginWaiting && a.Login != model.LoginReady {
		s.mu.Unlock()
		return a, ErrNoActiveLogin
	}
	if attempt := s.attempts[id]; attempt != nil {
		attempt.cancel()
		s.finishAttemptLocked(id, attempt)
	}

	originalCode := a.VerifyCode
	status := model.AccountOnline
	login := model.LoginDone
	code := originalCode
	if !ok {
		status = model.AccountAbnormal
		login = model.LoginFail
		code = ""
	}
	if err := s.repo.UpdateAccountLogin(ctx, id, status, login, a.LoginRule, code, a.VerifyPhone); err != nil {
		s.mu.Unlock()
		return a, err
	}
	s.mu.Unlock()

	a.Status = status
	a.Login = login
	a.VerifyCode = code

	if originalCode != "" {
		// Best effort: the verdict is already saved, and a missing SMS row
		// must not make the confirmation look failed. Marking it used on a
		// failure too prevents the stale code from being reused.
		_ = s.repo.MarkSmsUsed(ctx, a.VerifyPhone, originalCode)
	}
	s.notify(model.Change{Kind: "accounts", AccountID: id})
	if originalCode != "" {
		s.notify(model.Change{Kind: "sms", AccountID: id})
	}
	return a, nil
}

// cancelLogin stops an in-flight attempt for an account without touching
// its persisted state. Used when the account is deleted.
func (s *Service) cancelLogin(id int64) {
	s.mu.Lock()
	if attempt := s.attempts[id]; attempt != nil {
		attempt.cancel()
		s.finishAttemptLocked(id, attempt)
	}
	s.mu.Unlock()
}

func (s *Service) finishAttemptLocked(id int64, attempt *loginAttempt) {
	if s.attempts[id] == attempt {
		delete(s.attempts, id)
	}
	if attempt.taskID != 0 {
		s.finishTaskLocked(attempt.taskID)
	}
}

// RecoverStuckLogins repairs state left by a crash. Waiting attempts have no
// worker anymore, so they become failures; a ready code is kept because the
// user can still confirm an already received code.
func (s *Service) RecoverStuckLogins(ctx context.Context) error {
	accounts, err := s.repo.ListAccounts(ctx)
	if err != nil {
		return err
	}
	var errs []error
	for _, a := range accounts {
		if a.Login == model.LoginReady {
			continue
		}
		if a.Status != model.AccountLoggingIn && a.Login != model.LoginWaiting {
			continue
		}
		if err := s.repo.UpdateAccountLogin(ctx, a.ID, model.AccountAbnormal, model.LoginFail,
			a.LoginRule, "", a.VerifyPhone); err != nil {
			errs = append(errs, fmt.Errorf("account %d: %w", a.ID, err))
		}
	}
	return errors.Join(errs...)
}
