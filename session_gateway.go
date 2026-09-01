package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
)

type signupInput struct {
	TenantID     string `json:"tenant_id"`
	Email        string `json:"email"`
	Password     string `json:"password"`
	Name         string `json:"name"`
	CaptchaToken string `json:"captcha_token"`
	RequestID    string `json:"request_id"`
}

type loginInput struct {
	Email string `json:"email"`
}

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	auth := newTenantAuth(&APIClient{BaseURL: defaultBaseURL, Key: key})
	mux := http.NewServeMux()
	mux.HandleFunc("POST /signup", func(w http.ResponseWriter, r *http.Request) {
		var in signupInput
		if json.NewDecoder(r.Body).Decode(&in) != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		session, err := auth.signup(r.Context(), in.TenantID, in.Email, in.Password, in.Name, in.CaptchaToken, clientIP(r), in.RequestID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		setSessionCookie(w, session)
		writeJSON(w, http.StatusCreated, map[string]string{"tenant_id": in.TenantID, "account_state": string(accountActive)})
	})
	mux.HandleFunc("POST /login", func(w http.ResponseWriter, r *http.Request) {
		var in loginInput
		if json.NewDecoder(r.Body).Decode(&in) != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		session, err := auth.login(r.Context(), in.Email)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		setSessionCookie(w, session)
		writeJSON(w, http.StatusOK, map[string]string{"status": "signed_in"})
	})
	mux.HandleFunc("POST /admin/accounts/{email}/disable", func(w http.ResponseWriter, r *http.Request) {
		if err := auth.disableAccount(r.PathValue("email")); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"account_state": string(accountDisabled), "sessions": "revoked"})
	})
	mux.HandleFunc("GET /me", func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("saas_session")
		if err != nil {
			http.Error(w, "authentication required", http.StatusUnauthorized)
			return
		}
		acct, ok := auth.authenticate(cookie.Value)
		if !ok {
			http.Error(w, "authentication required", http.StatusUnauthorized)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"email": acct.Email, "tenant_id": acct.TenantID, "role": acct.Role})
	})
	addr := ":8080"
	log.Printf("tenant session gateway listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func setSessionCookie(w http.ResponseWriter, session browserSession) {
	http.SetCookie(w, &http.Cookie{Name: "saas_session", Value: session.Token, Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode, Expires: session.ExpiresAt})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}

func clientIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		return strings.TrimSpace(strings.Split(forwarded, ",")[0])
	}
	return r.RemoteAddr
}
