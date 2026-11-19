// Package model holds the domain types of the cloud-phone platform: the
// accounts it manages, the cloud phones (virtual machines) they live on,
// the SMS codes that verify logins, the SIM card slots, the users and
// their wallets, and the tasks the queue runs.
package model

import "time"

// AccountStatus is where an account is in its life cycle.
type AccountStatus string

const (
	AccountOffline   AccountStatus = "offline"    // logged out, not bound to a running phone
	AccountOnline    AccountStatus = "online"     // logged in on its cloud phone
	AccountAbnormal  AccountStatus = "abnormal"   // something needs attention: a failed login, a ban, a missing phone
	AccountLoggingIn AccountStatus = "logging_in" // a login task is running now
)

// LoginState is the finer-grained state of the last login attempt, shown in
// the console dialog.
type LoginState string

const (
	LoginNone    LoginState = ""
	LoginWaiting LoginState = "waiting" // waiting for the SMS code
	LoginReady   LoginState = "ready"   // the code arrived, waiting for the user to confirm
	LoginDone    LoginState = "done"    // the user confirmed success
	LoginFail    LoginState = "fail"    // the user confirmed failure, or the rule timed out
)

// Account is one platform account (e.g. a QQ or a game account) driven by a
// cloud phone.
type Account struct {
	ID         int64  `json:"id"`
	Remark     string `json:"remark"`
	Number     string `json:"number"`      // the account name / login id, e.g. 2441864785
	Display    string `json:"display"`     // the nickname shown, e.g. 祈
	BoundPhone string `json:"bound_phone"` // the cloud phone name, "" if none
	// BoundPhoneID is the primary key of the bound cloud phone. It is the
	// lookup key; BoundPhone is kept as the display name.
	BoundPhoneID int64         `json:"bound_phone_id"`
	Status       AccountStatus `json:"status"`
	// Login is the state of the console's current login attempt.
	Login       LoginState `json:"login"`
	LoginRule   string     `json:"login_rule"`   // e.g. qq_account_management_login_rule
	VerifyCode  string     `json:"verify_code"`  // the last code received
	VerifyPhone string     `json:"verify_phone"` // the number it came from, e.g. 135******81
	LinkedQQ    bool       `json:"linked_qq"`
	LinkedQID   string     `json:"linked_qid"` // "" when not enabled
	HasPassword bool       `json:"has_password"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// PhoneState is the state of a cloud phone.
type PhoneState string

const (
	PhoneIdle     PhoneState = "idle"     // 云机空闲, ready to be allocated
	PhoneRunning  PhoneState = "running"  // powered on, showing a screen
	PhoneStopped  PhoneState = "stopped"  // powered off
	PhoneStarting PhoneState = "starting" // booting
	PhoneError    PhoneState = "error"
)

// CloudPhone is a virtual Android phone in the cloud.
type CloudPhone struct {
	ID         int64      `json:"id"`
	Name       string     `json:"name"`        // e.g. 24418647
	ExternalID string     `json:"external_id"` // the provider's device id; Name when empty
	Region     string     `json:"region"`      // e.g. 华东-1
	Image      string     `json:"image"`       // e.g. Android 12
	State      PhoneState `json:"state"`
	AccountID  int64      `json:"account_id"` // 0 when unallocated
	IP         string     `json:"ip"`
}

// Sms is a received verification message.
type Sms struct {
	ID         int64     `json:"id"`
	Phone      string    `json:"phone"`
	Code       string    `json:"code"`
	Content    string    `json:"content"`
	ReceivedAt time.Time `json:"received_at"`
	Used       bool      `json:"used"`
}

// SimSlot is a SIM card position holding a phone number.
type SimSlot struct {
	ID           int64  `json:"id"`
	Phone        string `json:"phone"`
	Carrier      string `json:"carrier"` // 移动 / 联通 / 电信
	BoundAccount int64  `json:"bound_account"`
	Status       string `json:"status"` // 空闲 / 占用 / 停机
}

// User is the operator of the platform.
type User struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Role     string `json:"role"` // 管理员 / 运营 / 只读
	Enabled  bool   `json:"enabled"`
}

// Wallet is the platform balance, in yuan.
type Wallet struct {
	Balance float64 `json:"balance"`
}

// LedgerEntry is one movement of the wallet.
type LedgerEntry struct {
	ID     int64     `json:"id"`
	At     time.Time `json:"at"`
	Amount float64   `json:"amount"` // positive is a top-up, negative a charge
	Reason string    `json:"reason"`
}

// TaskKind is what a queued task does.
type TaskKind string

const (
	TaskLogin TaskKind = "login" // 登扣
	TaskBoot  TaskKind = "boot"
)

// Task is one unit of work the queue runs, shown in the "进行中任务" bar.
type Task struct {
	ID        int64     `json:"id"`
	Kind      TaskKind  `json:"kind"`
	AccountID int64     `json:"account_id"`
	Label     string    `json:"label"`
	StartedAt time.Time `json:"started_at"`
	Done      bool      `json:"done"`
}
