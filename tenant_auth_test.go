package main

import (
	"context"
	"testing"
	"time"
)

type fakeAuthAPI struct{}

func (fakeAuthAPI) VerifyCaptcha(context.Context, string, string) error { return nil }
func (fakeAuthAPI) CreateUser(context.Context, string, string, string, string, string) (createdUser, error) {
	return createdUser{ID: "user-42"}, nil
}
func (fakeAuthAPI) CreateSession(context.Context, string) (remoteSession, error) {
	return remoteSession{ID: "remote-session-7"}, nil
}

func TestAccountLifecycleControlsSessions(t *testing.T) {
	tests := []struct {
		name       string
		disable    bool
		wantAccess bool
	}{
		{name: "active owner keeps access", wantAccess: true},
		{name: "disabled owner loses every session", disable: true, wantAccess: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth := newTenantAuth(fakeAuthAPI{})
			auth.now = func() time.Time { return time.Unix(1_700_000_000, 0) }
			session, err := auth.signup(context.Background(), "tenant-acme", "owner@acme.test", "long-password", "Ada", "captcha-value", "127.0.0.1", "signup-001")
			if err != nil {
				t.Fatal(err)
			}
			if tt.disable {
				if err := auth.disableAccount("owner@acme.test"); err != nil {
					t.Fatal(err)
				}
			}
			_, gotAccess := auth.authenticate(session.Token)
			if gotAccess != tt.wantAccess {
				t.Fatalf("access = %v, want %v", gotAccess, tt.wantAccess)
			}
		})
	}
}
