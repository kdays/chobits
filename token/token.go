package token

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/kdays/chobits/cache"
)

var (
	ErrInvalid = errors.New("invalid token")
	ErrExpired = errors.New("token expired")
)

type Claims struct {
	Subject  string            `json:"sub"`
	Kind     string            `json:"kind,omitempty"`
	Scopes   []string          `json:"scopes,omitempty"`
	IssuedAt int64             `json:"iat"`
	Expires  int64             `json:"exp"`
	Meta     map[string]string `json:"meta,omitempty"`
}

type Token struct {
	Value  string
	Claims Claims
}

type Service struct {
	store  cache.Cache
	prefix string
	ttl    time.Duration
	now    func() time.Time
}

type Option func(*Service)

func WithPrefix(prefix string) Option {
	return func(service *Service) {
		service.prefix = prefix
	}
}

func WithTTL(ttl time.Duration) Option {
	return func(service *Service) {
		service.ttl = ttl
	}
}

func New(store cache.Cache, opts ...Option) *Service {
	if store == nil {
		store = cache.NewMemory()
	}
	service := &Service{
		store:  store,
		prefix: "cbt_",
		ttl:    120 * time.Hour,
		now:    time.Now,
	}
	for _, opt := range opts {
		opt(service)
	}
	return service
}

func (service *Service) Issue(ctx context.Context, claims Claims) (*Token, error) {
	ctx = ensureContext(ctx)
	if strings.TrimSpace(claims.Subject) == "" {
		return nil, ErrInvalid
	}

	now := service.now()
	if claims.IssuedAt == 0 {
		claims.IssuedAt = now.Unix()
	}
	ttl := service.ttl
	if claims.Expires > 0 {
		ttl = time.Unix(claims.Expires, 0).Sub(now)
	} else {
		claims.Expires = now.Add(ttl).Unix()
	}
	if ttl <= 0 {
		return nil, ErrExpired
	}

	raw, err := service.newRawToken()
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(claims)
	if err != nil {
		return nil, err
	}
	if err := service.store.Set(ctx, service.key(raw), data, ttl); err != nil {
		return nil, err
	}
	return &Token{Value: raw, Claims: claims}, nil
}

func (service *Service) IssueSubject(ctx context.Context, subject string) (*Token, error) {
	return service.Issue(ctx, Claims{Subject: subject})
}

func (service *Service) Find(ctx context.Context, raw string) (*Claims, error) {
	ctx = ensureContext(ctx)
	if strings.TrimSpace(raw) == "" {
		return nil, ErrInvalid
	}
	data, err := service.store.Get(ctx, service.key(raw))
	if err != nil {
		return nil, err
	}
	var claims Claims
	if err := json.Unmarshal(data, &claims); err != nil {
		return nil, err
	}
	if claims.Expires > 0 && service.now().Unix() >= claims.Expires {
		_ = service.Revoke(ctx, raw)
		return nil, ErrExpired
	}
	return &claims, nil
}

func (service *Service) Revoke(ctx context.Context, raw string) error {
	ctx = ensureContext(ctx)
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	return service.store.Delete(ctx, service.key(raw))
}

func (service *Service) Close() error {
	return nil
}

func (service *Service) newRawToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return service.prefix + base64.RawURLEncoding.EncodeToString(buf), nil
}

func (service *Service) key(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return "token:" + hex.EncodeToString(sum[:])
}

func ensureContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
