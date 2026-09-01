package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

type authAPI interface {
	VerifyCaptcha(context.Context, string, string) error
	CreateUser(context.Context, string, string, string, string, string) (createdUser, error)
	CreateSession(context.Context, string) (remoteSession, error)
}

type accountState string

const (
	accountActive   accountState = "active"
	accountDisabled accountState = "disabled"
)

type account struct {
	UserID   string
	TenantID string
	Email    string
	Role     string
	State    accountState
}

type browserSession struct {
	Token     string
	RemoteID  string
	UserID    string
	ExpiresAt time.Time
}

type tenantAuth struct {
	api      authAPI
	now      func() time.Time
	mu       sync.Mutex
	accounts map[string]account
	sessions map[string]browserSession
}

func newTenantAuth(api authAPI) *tenantAuth {
	return &tenantAuth{api: api, now: time.Now, accounts: make(map[string]account), sessions: make(map[string]browserSession)}
}

func (a *tenantAuth) signup(ctx context.Context, tenantID, email, password, name, captcha, ip, requestID string) (browserSession, error) {
	if tenantID == "" || email == "" || password == "" || captcha == "" || requestID == "" {
		return browserSession{}, errors.New("tenant_id, email, password, captcha_token and request_id are required")
	}
	if err := a.api.VerifyCaptcha(ctx, captcha, ip); err != nil {
		return browserSession{}, err
	}
	user, err := a.api.CreateUser(ctx, email, password, name, tenantID, requestID)
	if err != nil {
		return browserSession{}, err
	}
	a.mu.Lock()
	a.accounts[email] = account{UserID: user.ID, TenantID: tenantID, Email: email, Role: "owner", State: accountActive}
	a.mu.Unlock()
	return a.login(ctx, email)
}

func (a *tenantAuth) login(ctx context.Context, email string) (browserSession, error) {
	a.mu.Lock()
	acct, found := a.accounts[email]
	a.mu.Unlock()
	if !found || acct.State != accountActive {
		return browserSession{}, errors.New("account is not active")
	}
	remote, err := a.api.CreateSession(ctx, acct.UserID)
	if err != nil {
		return browserSession{}, err
	}
	token, err := randomToken()
	if err != nil {
		return browserSession{}, err
	}
	session := browserSession{Token: token, RemoteID: remote.ID, UserID: acct.UserID, ExpiresAt: a.now().Add(8 * time.Hour)}
	a.mu.Lock()
	a.sessions[token] = session
	a.mu.Unlock()
	return session, nil
}

func (a *tenantAuth) disableAccount(email string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	acct, found := a.accounts[email]
	if !found {
		return errors.New("account not found")
	}
	acct.State = accountDisabled
	a.accounts[email] = acct
	for token, session := range a.sessions {
		if session.UserID == acct.UserID {
			delete(a.sessions, token)
		}
	}
	return nil
}

func (a *tenantAuth) authenticate(token string) (account, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	session, found := a.sessions[token]
	if !found || !a.now().Before(session.ExpiresAt) {
		delete(a.sessions, token)
		return account{}, false
	}
	for _, acct := range a.accounts {
		if acct.UserID == session.UserID && acct.State == accountActive {
			return acct, true
		}
	}
	return account{}, false
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
