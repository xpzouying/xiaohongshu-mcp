package main

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/xpzouying/headless_browser"
	"github.com/xpzouying/xiaohongshu-mcp/account"
	"github.com/xpzouying/xiaohongshu-mcp/xiaohongshu"
)

type browserAccountLogin struct {
	cookies  account.CookieStore
	registry account.Registry
	locks    account.LockManager
	create   func(string) account.Browser
	mu       sync.Mutex
	sessions map[string]uint64
}

func newBrowserAccountLogin(cookies account.CookieStore, registry account.Registry, locks account.LockManager) AccountLogin {
	return &browserAccountLogin{cookies: cookies, registry: registry, locks: locks, create: newBrowserWithAccountCookie, sessions: make(map[string]uint64)}
}

func (l *browserAccountLogin) open(accountID string) (*headless_browser.Browser, error) {
	path, err := l.cookies.Path(accountID)
	if err != nil {
		return nil, err
	}
	browser, ok := l.create(path).(*headless_browser.Browser)
	if !ok {
		return nil, &account.Error{Code: account.CodeInternalError, Message: "账号浏览器类型无效"}
	}
	return browser, nil
}

func (l *browserAccountLogin) Status(ctx context.Context, accountID string) (bool, string, error) {
	browser, err := l.open(accountID)
	if err != nil {
		return false, "", err
	}
	defer browser.Close()
	page := browser.NewPage()
	defer page.Close()
	loggedIn, err := xiaohongshu.NewLogin(page).CheckLoginStatus(ctx)
	return loggedIn, "", err
}

func (l *browserAccountLogin) QRCode(ctx context.Context, accountID string) (string, bool, error) {
	browser, err := l.open(accountID)
	if err != nil {
		return "", false, err
	}
	page := browser.NewPage()
	login := xiaohongshu.NewLogin(page)
	image, loggedIn, err := login.FetchQrcodeImage(ctx)
	if err != nil || loggedIn {
		_ = page.Close()
		browser.Close()
		return image, loggedIn, err
	}
	generation := l.beginSession(accountID)
	go func() {
		defer page.Close()
		defer browser.Close()
		waitCtx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
		defer cancel()
		if !login.WaitForLogin(waitCtx) {
			return
		}
		cookies, cookieErr := page.Browser().GetCookies()
		if cookieErr != nil {
			return
		}
		data, marshalErr := json.Marshal(cookies)
		if marshalErr != nil {
			return
		}
		if err := l.persistLogin(context.Background(), accountID, generation, data); err != nil {
			logrus.WithError(err).Warnf("failed to persist QR login for account %q", accountID)
		}
	}()
	return image, false, nil
}

func (l *browserAccountLogin) beginSession(accountID string) uint64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sessions[accountID]++
	return l.sessions[accountID]
}

func (l *browserAccountLogin) Cancel(accountID string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sessions[accountID]++
}

func (l *browserAccountLogin) persistLogin(ctx context.Context, accountID string, generation uint64, data []byte) error {
	release, err := l.locks.Acquire(ctx, accountID)
	if err != nil {
		return err
	}
	defer release()
	l.mu.Lock()
	currentGeneration := l.sessions[accountID]
	l.mu.Unlock()
	if currentGeneration != generation {
		return &account.Error{Code: account.CodeOperationCanceled, Message: "二维码登录会话已失效"}
	}
	current, err := l.registry.Get(ctx, accountID)
	if err != nil {
		return err
	}
	if current.Status != account.StatusNeedsLogin {
		return &account.Error{Code: account.CodeAccountLoginRequired, Message: "账号登录状态已变更"}
	}
	if err := l.cookies.Save(ctx, accountID, data); err != nil {
		return err
	}
	if err := l.registry.UpdateStatus(ctx, accountID, account.StatusActive, "二维码登录成功"); err != nil {
		return errors.Join(err, l.cookies.Delete(context.WithoutCancel(ctx), accountID))
	}
	return nil
}
