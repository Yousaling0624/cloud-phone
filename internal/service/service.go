// Package service holds the platform's business logic: accounts, login
// orchestration, cloud phones, the wallet, settings, and the task queue.
package service

import (
	"context"
	"sort"
	"sync"

	"cloud-phone/internal/driver"
	"cloud-phone/internal/model"
	"cloud-phone/internal/store"
)

// loginAttempt is one running login, guarded by Service.mu.
type loginAttempt struct {
	gen    uint64
	cancel context.CancelFunc
	taskID int64
	opts   model.LoginOptions
}

// Service is the app's business layer over a repository and the drivers.
type Service struct {
	repo  store.Repository
	phone driver.PhoneDriver
	sms   driver.SmsDriver
	login driver.LoginDriver

	rootCtx    context.Context
	rootCancel context.CancelFunc
	wg         sync.WaitGroup

	mu       sync.Mutex // attempts, tasks, nextGen, nextTask, closing
	attempts map[int64]*loginAttempt
	nextGen  uint64
	tasks    map[int64]*model.Task
	nextTask int64
	closing  bool

	accountMu   sync.Mutex // import dedupe
	bindMu      sync.Mutex // binding validate/write
	walletMu    sync.Mutex // wallet read/modify/write
	phoneSyncMu sync.Mutex // provider device sync

	notifyMu sync.RWMutex
	notifyFn func(model.Change)
}

// New wires a service. The drivers may each be a real gateway or a mock.
func New(repo store.Repository, phone driver.PhoneDriver, sms driver.SmsDriver, login driver.LoginDriver) *Service {
	ctx, cancel := context.WithCancel(context.Background())
	return &Service{
		repo: repo, phone: phone, sms: sms, login: login,
		rootCtx: ctx, rootCancel: cancel,
		attempts: map[int64]*loginAttempt{},
		tasks:    map[int64]*model.Task{},
	}
}

// Repo exposes the repository for read-only queries.
func (s *Service) Repo() store.Repository { return s.repo }

// SetNotifier installs the callback that announces changes. The server
// points it at its SSE hub; tests leave it nil.
func (s *Service) SetNotifier(fn func(model.Change)) {
	s.notifyMu.Lock()
	s.notifyFn = fn
	s.notifyMu.Unlock()
}

// notify must never be called while s.mu, bindMu or walletMu is held.
func (s *Service) notify(c model.Change) {
	s.notifyMu.RLock()
	fn := s.notifyFn
	s.notifyMu.RUnlock()
	if fn != nil {
		fn(c)
	}
}

// Close cancels running logins and waits for their goroutines. Login
// goroutines do not write to the database while closing.
func (s *Service) Close() {
	s.mu.Lock()
	if s.closing {
		s.mu.Unlock()
		return
	}
	s.closing = true
	s.mu.Unlock()
	s.rootCancel()
	s.wg.Wait()
}

// --- tasks ---

func (s *Service) addTaskLocked(t model.Task) *model.Task {
	s.nextTask++
	t.ID = s.nextTask
	cp := t
	s.tasks[t.ID] = &cp
	return &cp
}

func (s *Service) finishTaskLocked(id int64) {
	delete(s.tasks, id)
}

// Tasks lists the running tasks sorted by id, for the status bar.
func (s *Service) Tasks() []model.Task {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]model.Task, 0, len(s.tasks))
	for _, t := range s.tasks {
		out = append(out, *t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// --- phones ---

// Phones lists cloud phones.
func (s *Service) Phones(ctx context.Context) ([]model.CloudPhone, error) {
	return s.repo.ListPhones(ctx)
}

// SetPhoneState runs the driver then saves the phone's new state.
func (s *Service) SetPhoneState(ctx context.Context, id int64, action string) error {
	p, err := s.repo.GetPhone(ctx, id)
	if err != nil {
		return wrapNotFound("云机", id, err)
	}
	var state model.PhoneState
	switch action {
	case "start":
		err = s.phone.Start(ctx, p)
		state = model.PhoneRunning
	case "stop":
		err = s.phone.Stop(ctx, p)
		state = model.PhoneStopped
	case "restart":
		err = s.phone.Restart(ctx, p)
		state = model.PhoneRunning
	default:
		return errUnknownAction(action)
	}
	if err != nil {
		return err
	}
	if err := s.repo.UpdatePhoneState(ctx, id, state); err != nil {
		return err
	}
	s.notify(model.Change{Kind: "phones"})
	return nil
}

// SetUserEnabled turns an operator account on or off and persists it.
func (s *Service) SetUserEnabled(ctx context.Context, id int64, enabled bool) error {
	users, err := s.repo.ListUsers(ctx)
	if err != nil {
		return err
	}
	for _, u := range users {
		if u.ID != id {
			continue
		}
		u.Enabled = enabled
		if err := s.repo.UpdateUser(ctx, u); err != nil {
			return err
		}
		s.notify(model.Change{Kind: "users"})
		return nil
	}
	return errNotFound("用户", id)
}

// Screenshot resolves the account's bound phone and returns its screen.
func (s *Service) Screenshot(ctx context.Context, accountID int64) ([]byte, error) {
	a, err := s.repo.GetAccount(ctx, accountID)
	if err != nil {
		return nil, wrapNotFound("账号", accountID, err)
	}
	phone, err := s.phoneForAccount(ctx, a)
	if err != nil {
		return nil, err
	}
	data, err := s.phone.Screenshot(ctx, phone)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (s *Service) phoneForAccount(ctx context.Context, a model.Account) (model.CloudPhone, error) {
	if a.BoundPhoneID != 0 {
		p, err := s.repo.GetPhone(ctx, a.BoundPhoneID)
		if err == nil {
			return p, nil
		}
		if !isNotFound(err) {
			return model.CloudPhone{}, err
		}
	}
	if a.BoundPhone != "" {
		phones, err := s.repo.ListPhones(ctx)
		if err != nil {
			return model.CloudPhone{}, err
		}
		for _, p := range phones {
			if p.Name == a.BoundPhone {
				return p, nil
			}
		}
	}
	return model.CloudPhone{}, ErrNoPhone
}
