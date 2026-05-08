package kdaysuc

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var ErrUnavailable = errors.New("kdays uc service unavailable")

type Authorizer interface {
	SessionAccessToken(ctx context.Context, accessCode string, expireIn int) (string, error)
	UserInfo(ctx context.Context, accessToken string) (UserInfo, error)
}

type Config struct {
	APIHost        string        `yaml:"api_host"`
	PublicHost     string        `yaml:"public_host"`
	APIKey         string        `yaml:"api_key"`
	Secret         string        `yaml:"secret"`
	AppID          string        `yaml:"app_id"`
	TimeoutSeconds int           `yaml:"timeout_seconds"`
	Timeout        time.Duration `yaml:"-"`
}

type UserInfo struct {
	OpenID      string `json:"open_id"`
	UnionID     string `json:"union_id"`
	UserNick    string `json:"user_nick"`
	Nick        string `json:"nick,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
	UserEmail   string `json:"user_email"`
	Email       string `json:"email,omitempty"`
	AvatarURL   string `json:"avatar_url"`
	Avatar      string `json:"avatar,omitempty"`
	UserSalt    string `json:"user_salt"`
	Salt        string `json:"salt,omitempty"`
	UserPasswd  string `json:"user_password"`
	Password    string `json:"password,omitempty"`
	ExpiresIn   int    `json:"expires_in,omitempty"`
	PushKey     string `json:"push_key,omitempty"`
}

func (info UserInfo) StableOpenID() string {
	if info.UnionID != "" {
		return info.UnionID
	}
	return info.OpenID
}

type LoginResponse struct {
	AccessToken string `json:"access_token"`
}

type Client struct {
	cfg        Config
	httpClient *http.Client
	now        func() time.Time
}

func New(cfg Config) *Client {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = time.Duration(cfg.TimeoutSeconds) * time.Second
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &Client{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: timeout},
		now:        time.Now,
	}
}

func NewClient(host, apiKey, secret string) *Client {
	return New(Config{
		APIHost: host,
		APIKey:  apiKey,
		Secret:  secret,
	})
}

func (client *Client) SessionAccessToken(ctx context.Context, accessCode string, expireIn int) (string, error) {
	if expireIn <= 0 {
		expireIn = 3600
	}
	var payload LoginResponse
	if err := client.Call(ctx, http.MethodPost, "/oauth/token", map[string]any{
		"code":      accessCode,
		"expire_in": expireIn,
	}, "", &payload); err != nil {
		return "", err
	}
	if payload.AccessToken == "" {
		return "", fmt.Errorf("kdays uc /oauth/token missing access_token")
	}
	return payload.AccessToken, nil
}

func (client *Client) AccessToken(ctx context.Context, accessCode string, expireIn int) (string, error) {
	return client.SessionAccessToken(ctx, accessCode, expireIn)
}

func (client *Client) GetAccessToken(accessCode string, expireIn ...int) (string, error) {
	exp := 3600
	if len(expireIn) > 0 {
		exp = expireIn[0]
	}
	return client.SessionAccessToken(context.Background(), accessCode, exp)
}

func (client *Client) UserInfo(ctx context.Context, accessToken string) (UserInfo, error) {
	var payload UserInfo
	if err := client.Call(ctx, http.MethodGet, "/oauth/me", nil, accessToken, &payload); err != nil {
		return UserInfo{}, err
	}
	return payload, nil
}

func (client *Client) GetUserInfo(accessToken string) (*UserInfo, error) {
	info, err := client.UserInfo(context.Background(), accessToken)
	if err != nil {
		return nil, err
	}
	return &info, nil
}

func (client *Client) LoginURL(state string) string {
	return LoginURL(client.cfg.PublicHost, client.cfg.AppID, state)
}

func LoginURL(host string, clientID string, state string) string {
	parsed, err := url.Parse(strings.TrimRight(host, "/"))
	if err != nil {
		return ""
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/sso/login/"
	query := parsed.Query()
	if clientID != "" {
		query.Set("client_id", clientID)
	}
	if state != "" {
		query.Set("state", state)
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func (client *Client) Call(ctx context.Context, method string, action string, payload any, accessToken string, target any) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if client == nil || client.cfg.APIHost == "" || client.cfg.APIKey == "" || client.cfg.Secret == "" {
		return ErrUnavailable
	}

	bodyText := "[]"
	if payload != nil {
		body, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		bodyText = string(body)
	}

	signTime := fmt.Sprintf("%d", client.now().Unix())
	signHash := sha1.Sum([]byte(bodyText + client.cfg.Secret + signTime))

	var bodyReader io.Reader
	if method != http.MethodGet {
		bodyReader = bytes.NewReader([]byte(bodyText))
	}
	request, err := http.NewRequestWithContext(ctx, method, client.endpoint(action), bodyReader)
	if err != nil {
		return err
	}
	if method != http.MethodGet {
		request.Header.Set("Content-Type", "application/json")
	}
	request.Header.Set("X-API-KEY", client.cfg.APIKey)
	request.Header.Set("X-SIGN-TIME", signTime)
	request.Header.Set("X-SIGN", hex.EncodeToString(signHash[:]))
	if accessToken != "" {
		request.Header.Set("X-TOKEN", accessToken)
	}

	response, err := client.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("kdays uc network error: %w", err)
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 1024*1024))
	if err != nil {
		return fmt.Errorf("kdays uc %s read response: %w", action, err)
	}
	var envelope struct {
		Code int             `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}
	if len(responseBody) > 0 {
		if err := json.Unmarshal(responseBody, &envelope); err != nil {
			return fmt.Errorf("kdays uc %s invalid json response: http_status=%d: %w", action, response.StatusCode, err)
		}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return callError(action, response.StatusCode, envelope.Code, envelope.Msg)
	}
	if envelope.Code != 0 {
		return callError(action, response.StatusCode, envelope.Code, envelope.Msg)
	}
	if target != nil {
		if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
			return fmt.Errorf("kdays uc %s missing data", action)
		}
		if err := json.Unmarshal(envelope.Data, target); err != nil {
			return err
		}
	}
	return nil
}

func callError(action string, statusCode int, code int, message string) error {
	if message == "" {
		message = "kdays_uc_error"
	}
	return fmt.Errorf("kdays uc %s failed: http_status=%d code=%d msg=%s", action, statusCode, code, message)
}

func (client *Client) endpoint(action string) string {
	host := strings.TrimRight(client.cfg.APIHost, "/")
	return host + "/api/v1" + action
}

func (info *UserInfo) UnmarshalJSON(data []byte) error {
	type alias struct {
		OpenID       string `json:"open_id"`
		UnionID      string `json:"union_id"`
		UserNick     string `json:"user_nick"`
		DisplayName  string `json:"display_name"`
		UserEmail    string `json:"user_email"`
		Email        string `json:"email"`
		AvatarURL    string `json:"avatar_url"`
		UserAvatar   string `json:"user_avatar"`
		UserPassword string `json:"user_password"`
		UserPwd      string `json:"user_pwd"`
		UserSalt     string `json:"user_salt"`
		ExpiresIn    int    `json:"expires_in"`
		PushKey      string `json:"push_key"`
	}

	var raw alias
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	info.OpenID = raw.OpenID
	info.UnionID = raw.UnionID
	info.UserNick = firstNonEmpty(raw.UserNick, raw.DisplayName)
	info.Nick = info.UserNick
	info.DisplayName = firstNonEmpty(raw.DisplayName, raw.UserNick)
	info.UserEmail = firstNonEmpty(raw.UserEmail, raw.Email)
	info.Email = info.UserEmail
	info.AvatarURL = firstNonEmpty(raw.AvatarURL, raw.UserAvatar)
	info.Avatar = info.AvatarURL
	info.UserPasswd = firstNonEmpty(raw.UserPassword, raw.UserPwd)
	info.Password = info.UserPasswd
	info.UserSalt = raw.UserSalt
	info.Salt = raw.UserSalt
	info.ExpiresIn = raw.ExpiresIn
	info.PushKey = raw.PushKey

	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
