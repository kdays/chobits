package kdaysuc

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestUserInfoUnmarshalJSONSupportsNewFields(t *testing.T) {
	t.Parallel()

	payload := []byte(`{
		"avatar_url":"https://avatar.ikdays.com/1/189796_1672651442.jpg",
		"display_name":"amuse-bot",
		"email":"amuse@kdays.net",
		"expires_in":3599,
		"open_id":"MDE5MDgyNng2",
		"push_key":"P82TDnk1BaZI0gNWElQ48d1kVxY",
		"union_id":"MTE5MTg0NC5rZGF5cw",
		"user_email":"amuse@kdays.net",
		"user_password":"hashed-password",
		"user_salt":"Tz7x4x"
	}`)

	var info UserInfo
	if err := json.Unmarshal(payload, &info); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if info.UserNick != "amuse-bot" || info.Nick != "amuse-bot" {
		t.Fatalf("nick = %q/%q, want amuse-bot", info.UserNick, info.Nick)
	}
	if info.UserEmail != "amuse@kdays.net" || info.Email != "amuse@kdays.net" {
		t.Fatalf("email = %q/%q, want amuse@kdays.net", info.UserEmail, info.Email)
	}
	if info.PushKey != "P82TDnk1BaZI0gNWElQ48d1kVxY" {
		t.Fatalf("PushKey = %q", info.PushKey)
	}
	if info.UserSalt != "Tz7x4x" || info.Salt != "Tz7x4x" {
		t.Fatalf("salt = %q/%q, want Tz7x4x", info.UserSalt, info.Salt)
	}
	if info.ExpiresIn != 3599 {
		t.Fatalf("ExpiresIn = %d, want 3599", info.ExpiresIn)
	}
}

func TestUserInfoUnmarshalJSONKeepsLegacyFields(t *testing.T) {
	t.Parallel()

	payload := []byte(`{
		"avatar_url":"https://avatar.ikdays.com/1/legacy.jpg",
		"open_id":"open-id",
		"union_id":"union-id",
		"user_email":"legacy@kdays.net",
		"user_nick":"legacy-user",
		"user_password":"legacy-password",
		"user_salt":"legacy-salt"
	}`)

	var info UserInfo
	if err := json.Unmarshal(payload, &info); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if info.UserNick != "legacy-user" || info.Nick != "legacy-user" {
		t.Fatalf("nick = %q/%q, want legacy-user", info.UserNick, info.Nick)
	}
	if info.UserEmail != "legacy@kdays.net" || info.Email != "legacy@kdays.net" {
		t.Fatalf("email = %q/%q, want legacy@kdays.net", info.UserEmail, info.Email)
	}
	if info.UserSalt != "legacy-salt" || info.Salt != "legacy-salt" {
		t.Fatalf("salt = %q/%q, want legacy-salt", info.UserSalt, info.Salt)
	}
}

func TestLoginURL(t *testing.T) {
	t.Parallel()

	got := LoginURL("https://uc.example.test/base/", "app-id", "/next")
	if !strings.HasPrefix(got, "https://uc.example.test/base/sso/login/?") {
		t.Fatalf("login url = %q", got)
	}
	if !strings.Contains(got, "client_id=app-id") || !strings.Contains(got, "state=%2Fnext") {
		t.Fatalf("login url missing query: %q", got)
	}
}

func TestClientSessionAccessTokenAndUserInfo(t *testing.T) {
	client := New(Config{
		APIHost: "https://uc.example.test",
		APIKey:  "app-key",
		Secret:  "secret",
		Timeout: time.Second,
	})
	client.now = func() time.Time { return time.Unix(1700000000, 0) }
	client.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Header.Get("X-API-KEY") != "app-key" {
			t.Fatalf("X-API-KEY = %q, want app-key", req.Header.Get("X-API-KEY"))
		}
		if req.Header.Get("X-SIGN-TIME") != "1700000000" {
			t.Fatalf("X-SIGN-TIME = %q, want fixed timestamp", req.Header.Get("X-SIGN-TIME"))
		}
		if req.Header.Get("X-SIGN") == "" {
			t.Fatalf("missing X-SIGN")
		}

		switch req.URL.Path {
		case "/api/v1/oauth/token":
			if req.Method != http.MethodPost {
				t.Fatalf("token method = %s, want POST", req.Method)
			}
			return jsonResponse(`{"code":0,"data":{"access_token":"session-token"}}`), nil
		case "/api/v1/oauth/me":
			if req.Method != http.MethodGet {
				t.Fatalf("user info method = %s, want GET", req.Method)
			}
			if req.Header.Get("Content-Type") != "" {
				t.Fatalf("GET Content-Type = %q, want empty", req.Header.Get("Content-Type"))
			}
			if req.Header.Get("X-TOKEN") != "session-token" {
				t.Fatalf("X-TOKEN = %q, want session-token", req.Header.Get("X-TOKEN"))
			}
			return jsonResponse(`{"code":0,"data":{"avatar_url":"https://avatar.example.test/1.jpg","display_name":"Display User","email":"display@example.test","open_id":"openid-1","union_id":"unionid-1","user_password":"hashed-password","user_salt":"salt"}}`), nil
		default:
			t.Fatalf("unexpected path: %s", req.URL.Path)
			return nil, nil
		}
	})}

	sessionToken, err := client.SessionAccessToken(context.Background(), "access-code", 3600)
	if err != nil {
		t.Fatalf("session access token: %v", err)
	}
	if sessionToken != "session-token" {
		t.Fatalf("session token = %q, want session-token", sessionToken)
	}

	userInfo, err := client.UserInfo(context.Background(), sessionToken)
	if err != nil {
		t.Fatalf("user info: %v", err)
	}
	if userInfo.StableOpenID() != "unionid-1" || userInfo.Nick != "Display User" || userInfo.Avatar != "https://avatar.example.test/1.jpg" || userInfo.Password != "hashed-password" || userInfo.Salt != "salt" {
		t.Fatalf("unexpected user info: %#v", userInfo)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func jsonResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
