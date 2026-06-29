package todo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func (a *Service) readTodoTokenStore() todoTokenStore {
	var store todoTokenStore
	b, err := os.ReadFile(a.todoTokenFile)
	if err != nil {
		return store
	}
	_ = json.Unmarshal(b, &store)
	return store
}
func (a *Service) writeTodoTokenStore(store todoTokenStore) error {
	b, err := json.MarshalIndent(store, "", " ")
	if err != nil {
		return err
	}
	tmp := a.todoTokenFile + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0600); err != nil {
		return err
	}
	_ = os.Chmod(tmp, 0600)
	return os.Rename(tmp, a.todoTokenFile)
}
func (a *Service) unlinkTodo() error {
	a.todoMu.Lock()
	if a.todoAuthCancel != nil {
		a.todoAuthCancel()
		a.todoAuthCancel = nil
	}
	a.todoAuthState = todoAuthPending{}
	a.todoMu.Unlock()
	if _, err := a.writeTodoSettings(func(todo map[string]any) { todo["syncMode"] = todoSyncLocal }); err != nil {
		return err
	}
	if err := os.Remove(a.todoTokenFile); err != nil && !os.IsNotExist(err) {
		return err
	}
	a.todoNotifyInboundScheduler()
	return nil
}
func todoFormPost(ctx context.Context, endpoint string, form url.Values) (map[string]any, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{Timeout: 20 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer res.Body.Close()
	var out map[string]any
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		out = map[string]any{}
	}
	return out, res.StatusCode, nil
}
func (a *Service) startTodoAuth(clientID string) (map[string]any, error) {
	clientID = strings.TrimSpace(clientID)
	if clientID == "" {
		clientID = a.todoClientID()
	}
	if clientID == "" {
		return nil, errors.New("Microsoft To Do client ID is required")
	}
	// Do not switch the device into Microsoft mode until device-code consent has
	// produced a durable token. A cancelled or abandoned link must leave the
	// local-first source mode untouched.
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	form := url.Values{"client_id": {clientID}, "scope": {todoScope}}
	payload, status, err := todoFormPost(ctx, "https://login.microsoftonline.com/common/oauth2/v2.0/devicecode", form)
	if err != nil {
		cancel()
		return nil, err
	}
	if status < 200 || status >= 300 {
		cancel()
		return nil, fmt.Errorf("Microsoft device-code request failed: %v", payload["error_description"])
	}
	deviceCode := jsonutil.StringValue(payload["device_code"])
	pending := todoAuthPending{State: "pending", UserCode: jsonutil.StringValue(payload["user_code"]), VerificationURI: jsonutil.StringValue(payload["verification_uri"]), ExpiresAt: time.Now().Add(time.Duration(jsonutil.Int(payload["expires_in"], 900)) * time.Second).UnixMilli()}
	a.todoMu.Lock()
	if a.todoAuthCancel != nil {
		a.todoAuthCancel()
	}
	a.todoAuthCancel = cancel
	a.todoAuthState = pending
	a.todoMu.Unlock()
	interval := time.Duration(clamp(jsonutil.Int(payload["interval"], 5), 5, 30)) * time.Second
	go a.pollTodoDeviceCode(ctx, clientID, deviceCode, interval)
	return map[string]any{"ok": true, "state": "pending", "userCode": pending.UserCode, "verificationUri": pending.VerificationURI, "expiresIn": jsonutil.Int(payload["expires_in"], 900), "interval": int(interval.Seconds())}, nil
}
func (a *Service) pollTodoDeviceCode(ctx context.Context, clientID, deviceCode string, interval time.Duration) {
	defer func() { a.todoMu.Lock(); a.todoAuthCancel = nil; a.todoMu.Unlock() }()
	endpoint := "https://login.microsoftonline.com/common/oauth2/v2.0/token"
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}
		form := url.Values{"grant_type": {"urn:ietf:params:oauth:grant-type:device_code"}, "client_id": {clientID}, "device_code": {deviceCode}}
		payload, status, err := todoFormPost(ctx, endpoint, form)
		if err != nil {
			a.setTodoAuthReason("network_error")
			continue
		}
		if status >= 200 && status < 300 {
			store := todoTokenStore{ClientID: clientID, AccessToken: jsonutil.StringValue(payload["access_token"]), RefreshToken: jsonutil.StringValue(payload["refresh_token"]), AccessExpiresAt: time.Now().Add(time.Duration(jsonutil.Int(payload["expires_in"], 3600)) * time.Second).UnixMilli(), Scopes: strings.Fields(jsonutil.StringValue(payload["scope"])), LinkedAt: time.Now().UnixMilli()}
			if err := a.writeTodoTokenStore(store); err != nil {
				a.setTodoAuthReason("token_store_failed")
				return
			}
			if _, err := a.writeTodoSettings(func(todo map[string]any) {
				todo["clientId"] = clientID
				todo["syncMode"] = todoSyncMicrosoft
			}); err != nil {
				_ = os.Remove(a.todoTokenFile)
				a.setTodoAuthReason("settings_store_failed")
				return
			}
			a.todoMu.Lock()
			a.todoAuthState = todoAuthPending{State: "linked"}
			a.todoMu.Unlock()
			// Discover real Microsoft lists immediately after linking. This is a
			// bounded one-time list delta, not a dashboard-startup request.
			go func() {
				refreshCtx, refreshCancel := context.WithTimeout(context.Background(), 45*time.Second)
				defer refreshCancel()
				_ = a.syncTodoListsNow(refreshCtx)
				a.todoEmit(map[string]any{"type": "list.refresh"})
				a.todoNotifyInboundScheduler()
			}()
			return
		}
		errCode := jsonutil.StringValue(payload["error"])
		switch errCode {
		case "authorization_pending":
			continue
		case "slow_down":
			interval += 5 * time.Second
			continue
		}
		a.setTodoAuthReason(errCode)
		return
	}
}
func (a *Service) setTodoAuthReason(reason string) {
	a.todoMu.Lock()
	st := a.todoAuthState
	st.State = "unlinked"
	st.Reason = reason
	a.todoAuthState = st
	a.todoMu.Unlock()
}
func (a *Service) refreshTodoToken(ctx context.Context) (todoTokenStore, error) {
	store := a.readTodoTokenStore()
	if store.RefreshToken == "" {
		return store, errors.New("Microsoft To Do is not linked")
	}
	if store.AccessToken != "" && store.AccessExpiresAt > time.Now().Add(60*time.Second).UnixMilli() {
		return store, nil
	}
	form := url.Values{"grant_type": {"refresh_token"}, "client_id": {store.ClientID}, "refresh_token": {store.RefreshToken}, "scope": {todoScope}}
	payload, status, err := todoFormPost(ctx, "https://login.microsoftonline.com/common/oauth2/v2.0/token", form)
	if err != nil {
		return store, err
	}
	if status < 200 || status >= 300 {
		if jsonutil.StringValue(payload["error"]) == "invalid_grant" {
			_ = os.Remove(a.todoTokenFile)
		}
		return store, fmt.Errorf("token refresh failed: %v", payload["error"])
	}
	store.AccessToken = jsonutil.StringValue(payload["access_token"])
	if rt := jsonutil.StringValue(payload["refresh_token"]); rt != "" {
		store.RefreshToken = rt
	}
	store.AccessExpiresAt = time.Now().Add(time.Duration(jsonutil.Int(payload["expires_in"], 3600)) * time.Second).UnixMilli()
	_ = a.writeTodoTokenStore(store)
	return store, nil
}
