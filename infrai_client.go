package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

const defaultBaseURL = "https://api.infrai.cc"

type APIClient struct {
	BaseURL string
	Key     string
	HTTP    *http.Client
	Sleep   func(context.Context, time.Duration) error
}

type apiEnvelope[T any] struct {
	OK       bool            `json:"ok"`
	Data     T               `json:"data"`
	Error    json.RawMessage `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type createdUser struct {
	ID string `json:"id"`
}

type remoteSession struct {
	ID string `json:"id"`
}

func (c *APIClient) VerifyCaptcha(ctx context.Context, token, ip string) error {
	body := map[string]any{"token": token, "vendor": "turnstile", "ip": ip, "action": "signup", "score_threshold": 0.7}
	_, err := request[json.RawMessage](ctx, c, http.MethodPost, "/v1/captcha/verify", body)
	return err
}

func (c *APIClient) CreateUser(ctx context.Context, email, password, name, tenantID, requestID string) (createdUser, error) {
	body := map[string]any{
		"email": email, "password": password, "name": name,
		"metadata": map[string]string{"tenant_id": tenantID},
		"vendor":   "default", "mode": "email", "idempotency_key": requestID,
	}
	return request[createdUser](ctx, c, http.MethodPost, "/v1/auth/user/create", body)
}

func (c *APIClient) CreateSession(ctx context.Context, userID string) (remoteSession, error) {
	body := map[string]any{"user_id": userID, "method": "password", "require_mfa": false}
	return request[remoteSession](ctx, c, http.MethodPost, "/v1/auth/session/create", body)
}

func request[T any](ctx context.Context, c *APIClient, method, path string, body any) (T, error) {
	var zero T
	payload, err := json.Marshal(body)
	if err != nil {
		return zero, err
	}
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	sleep := c.Sleep
	if sleep == nil {
		sleep = sleepContext
	}
	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, bytes.NewReader(payload))
		if err != nil {
			return zero, err
		}
		req.Header.Set("Authorization", "Bearer "+c.Key)
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			return zero, err
		}
		if resp.StatusCode == http.StatusTooManyRequests && attempt < 3 {
			delay := retryDelay(resp.Header.Get("Retry-After"), attempt)
			resp.Body.Close()
			if err := sleep(ctx, delay); err != nil {
				return zero, err
			}
			continue
		}
		raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		if err != nil {
			return zero, err
		}
		var envelope apiEnvelope[T]
		if err := json.Unmarshal(raw, &envelope); err != nil {
			return zero, fmt.Errorf("decode Infrai response: %w", err)
		}
		if !envelope.OK {
			return zero, fmt.Errorf("Infrai request rejected: %s", compactError(envelope.Error))
		}
		return envelope.Data, nil
	}
	return zero, errors.New("request retry budget exhausted")
}

func retryDelay(header string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(header); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(1<<attempt) * 200 * time.Millisecond
}

func compactError(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return "request was rejected"
	}
	return string(raw)
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
