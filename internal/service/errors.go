package service

import (
	"errors"
	"fmt"

	"cloud-phone/internal/store"
)

// Sentinel errors the HTTP server maps to status codes. Messages are shown
// to the operator as-is.
var (
	ErrNotFound           = errors.New("对象不存在")
	ErrDuplicate          = errors.New("账号已存在")
	ErrLoginInProgress    = errors.New("该账号正在登扣中，请等待验证码")
	ErrAwaitingConfirm    = errors.New("验证码已到达，请先确认或放弃本次登录")
	ErrPhoneBusy          = errors.New("云机已被其他账号绑定")
	ErrNoPhone            = errors.New("该账号未绑定云机")
	ErrNoActiveLogin      = errors.New("当前没有进行中的登录")
	ErrLoginNotReady      = errors.New("验证码尚未到达，不能确认成功")
	ErrInvalidLoginMethod = errors.New("不支持的登录方式")
	ErrInvalidAmount      = errors.New("金额无效")
	ErrInsufficientFunds  = errors.New("余额不足")
	ErrInvalidSettings    = errors.New("设置无效")
	ErrEmptyNumber        = errors.New("账号不能为空")
	ErrClosed             = errors.New("服务正在关闭，请稍后重试")
)

// wrapNotFound turns the store's not-found sentinel into the service one,
// keeping the object kind and id in the message.
func wrapNotFound(kind string, id int64, err error) error {
	if errors.Is(err, store.ErrNotFound) {
		return fmt.Errorf("%s %d: %w", kind, id, ErrNotFound)
	}
	return err
}

func errNotFound(kind string, id int64) error {
	return fmt.Errorf("%s %d: %w", kind, id, ErrNotFound)
}

func errUnknownAction(action string) error {
	return fmt.Errorf("未知操作 %q", action)
}

func isNotFound(err error) bool {
	return errors.Is(err, store.ErrNotFound) || errors.Is(err, ErrNotFound)
}
