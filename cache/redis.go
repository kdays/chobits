package cache

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

const incrementScript = `
local value = redis.call("INCRBY", KEYS[1], ARGV[1])
local ttl = tonumber(ARGV[2])
if ttl and ttl > 0 then
	redis.call("PEXPIRE", KEYS[1], ttl)
end
return value
`

const takeScript = `
local value = redis.call("GET", KEYS[1])
if not value then
	return nil
end
redis.call("DEL", KEYS[1])
return value
`

var ErrRedisNil = errors.New("redis: nil")

type RedisClient interface {
	Do(ctx context.Context, args ...any) (any, error)
	Close() error
}

type RedisConfig struct {
	Addr     string `yaml:"addr"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
	Prefix   string `yaml:"prefix"`
	Required bool   `yaml:"required"`
}

type Redis struct {
	client      RedisClient
	prefix      string
	closeClient bool
}

type redisConn struct {
	network  string
	addr     string
	username string
	password string
	db       int

	mu     sync.Mutex
	conn   net.Conn
	reader *bufio.Reader
}

func OpenRedis(ctx context.Context, cfg RedisConfig) (*Redis, error) {
	client, err := OpenRedisClient(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return NewOwnedRedis(client, cfg.Prefix), nil
}

func OpenRedisClient(ctx context.Context, cfg RedisConfig) (RedisClient, error) {
	if cfg.Addr == "" {
		cfg.Addr = "127.0.0.1:6379"
	}
	client := &redisConn{
		network:  "tcp",
		addr:     cfg.Addr,
		username: cfg.Username,
		password: cfg.Password,
		db:       cfg.DB,
	}
	if cfg.Required {
		if err := client.connect(contextOrBackground(ctx)); err != nil {
			return nil, err
		}
	}
	return client, nil
}

func NewRedis(client RedisClient, prefix string) *Redis {
	return &Redis{
		client: client,
		prefix: prefix,
	}
}

func NewOwnedRedis(client RedisClient, prefix string) *Redis {
	return &Redis{
		client:      client,
		prefix:      prefix,
		closeClient: true,
	}
}

func (cache *Redis) Get(ctx context.Context, key string) ([]byte, error) {
	value, err := cache.client.Do(ctx, "GET", cache.key(key))
	if err != nil {
		if errors.Is(err, ErrRedisNil) {
			return nil, ErrMiss
		}
		return nil, err
	}
	return redisBytes(value)
}

func (cache *Redis) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	args := []any{"SET", cache.key(key), value}
	if ttl > 0 {
		args = append(args, "PX", redisTTLMillis(ttl))
	}
	_, err := cache.client.Do(ctx, args...)
	return err
}

func (cache *Redis) Take(ctx context.Context, key string) ([]byte, error) {
	result, err := cache.client.Do(ctx, "EVAL", takeScript, 1, cache.key(key))
	if err != nil {
		if errors.Is(err, ErrRedisNil) {
			return nil, ErrMiss
		}
		return nil, err
	}
	return redisBytes(result)
}

func (cache *Redis) Exists(ctx context.Context, key string) (bool, error) {
	result, err := cache.client.Do(ctx, "EXISTS", cache.key(key))
	if err != nil {
		return false, err
	}
	return redisInt64(result) > 0, nil
}

func (cache *Redis) Increment(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	ttlMillis := redisTTLMillis(ttl)
	result, err := cache.client.Do(ctx, "EVAL", incrementScript, 1, cache.key(key), delta, ttlMillis)
	if err != nil {
		return 0, err
	}
	return redisInt64(result), nil
}

func (cache *Redis) TTL(ctx context.Context, key string) (time.Duration, error) {
	result, err := cache.client.Do(ctx, "PTTL", cache.key(key))
	if err != nil {
		return 0, err
	}
	switch ttl := redisInt64(result); ttl {
	case -2:
		return 0, ErrMiss
	case -1:
		return 0, nil
	default:
		return time.Duration(ttl) * time.Millisecond, nil
	}
}

func (cache *Redis) Delete(ctx context.Context, key string) error {
	_, err := cache.client.Do(ctx, "DEL", cache.key(key))
	return err
}

func (cache *Redis) Close() error {
	if !cache.closeClient {
		return nil
	}
	return cache.client.Close()
}

func (cache *Redis) Client() RedisClient {
	if cache == nil {
		return nil
	}
	return cache.client
}

func (cache *Redis) key(key string) string {
	return Key(cache.prefix, key)
}

func (client *redisConn) Do(ctx context.Context, args ...any) (any, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("redis command is empty")
	}
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	client.mu.Lock()
	defer client.mu.Unlock()

	if client.conn == nil {
		if err := client.connectLocked(ctx); err != nil {
			return nil, err
		}
	}
	result, err := client.doLocked(ctx, args...)
	if err != nil && !errors.Is(err, ErrRedisNil) && !isRedisCommandError(err) {
		client.closeLocked()
	}
	return result, err
}

func (client *redisConn) Close() error {
	client.mu.Lock()
	defer client.mu.Unlock()
	return client.closeLocked()
}

func (client *redisConn) connect(ctx context.Context) error {
	client.mu.Lock()
	defer client.mu.Unlock()
	return client.connectLocked(ctx)
}

func (client *redisConn) connectLocked(ctx context.Context) error {
	dialer := net.Dialer{}
	conn, err := dialer.DialContext(ctx, client.network, client.addr)
	if err != nil {
		return fmt.Errorf("dial redis: %w", err)
	}
	client.conn = conn
	client.reader = bufio.NewReader(conn)

	if client.password != "" {
		if client.username != "" {
			if _, err := client.doLocked(ctx, "AUTH", client.username, client.password); err != nil {
				client.closeLocked()
				return fmt.Errorf("auth redis: %w", err)
			}
		} else {
			if _, err := client.doLocked(ctx, "AUTH", client.password); err != nil {
				client.closeLocked()
				return fmt.Errorf("auth redis: %w", err)
			}
		}
	}
	if client.db != 0 {
		if _, err := client.doLocked(ctx, "SELECT", client.db); err != nil {
			client.closeLocked()
			return fmt.Errorf("select redis db: %w", err)
		}
	}
	if _, err := client.doLocked(ctx, "PING"); err != nil {
		client.closeLocked()
		return fmt.Errorf("ping redis: %w", err)
	}
	return nil
}

func (client *redisConn) doLocked(ctx context.Context, args ...any) (any, error) {
	if err := client.setDeadline(ctx); err != nil {
		return nil, err
	}
	if err := writeRedisCommand(client.conn, args); err != nil {
		return nil, err
	}
	return readRedisReply(client.reader, true)
}

func (client *redisConn) setDeadline(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if deadline, ok := ctx.Deadline(); ok {
		return client.conn.SetDeadline(deadline)
	}
	return client.conn.SetDeadline(time.Time{})
}

func (client *redisConn) closeLocked() error {
	if client.conn == nil {
		return nil
	}
	err := client.conn.Close()
	client.conn = nil
	client.reader = nil
	return err
}

func writeRedisCommand(conn net.Conn, args []any) error {
	if _, err := fmt.Fprintf(conn, "*%d\r\n", len(args)); err != nil {
		return err
	}
	for _, arg := range args {
		data := redisArgBytes(arg)
		if _, err := fmt.Fprintf(conn, "$%d\r\n", len(data)); err != nil {
			return err
		}
		if _, err := conn.Write(data); err != nil {
			return err
		}
		if _, err := conn.Write([]byte("\r\n")); err != nil {
			return err
		}
	}
	return nil
}

func redisArgBytes(arg any) []byte {
	switch value := arg.(type) {
	case nil:
		return nil
	case []byte:
		return value
	case string:
		return []byte(value)
	case int:
		return []byte(strconv.FormatInt(int64(value), 10))
	case int64:
		return []byte(strconv.FormatInt(value, 10))
	case int32:
		return []byte(strconv.FormatInt(int64(value), 10))
	case uint:
		return []byte(strconv.FormatUint(uint64(value), 10))
	case uint64:
		return []byte(strconv.FormatUint(value, 10))
	case time.Duration:
		return []byte(strconv.FormatInt(int64(value), 10))
	default:
		return []byte(fmt.Sprint(value))
	}
}

func readRedisReply(reader *bufio.Reader, nilAsError bool) (any, error) {
	prefix, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}
	switch prefix {
	case '+':
		return readRedisLine(reader)
	case '-':
		line, err := readRedisLine(reader)
		if err != nil {
			return nil, err
		}
		return nil, redisCommandError(line)
	case ':':
		line, err := readRedisLine(reader)
		if err != nil {
			return nil, err
		}
		return strconv.ParseInt(line, 10, 64)
	case '$':
		line, err := readRedisLine(reader)
		if err != nil {
			return nil, err
		}
		length, err := strconv.Atoi(line)
		if err != nil {
			return nil, err
		}
		if length < 0 {
			if nilAsError {
				return nil, ErrRedisNil
			}
			return nil, nil
		}
		buf := make([]byte, length+2)
		if _, err := io.ReadFull(reader, buf); err != nil {
			return nil, err
		}
		return buf[:length], nil
	case '*':
		line, err := readRedisLine(reader)
		if err != nil {
			return nil, err
		}
		length, err := strconv.Atoi(line)
		if err != nil {
			return nil, err
		}
		if length < 0 {
			if nilAsError {
				return nil, ErrRedisNil
			}
			return nil, nil
		}
		values := make([]any, length)
		for i := 0; i < length; i++ {
			value, err := readRedisReply(reader, false)
			if err != nil {
				return nil, err
			}
			values[i] = value
		}
		return values, nil
	case '_':
		if _, err := readRedisLine(reader); err != nil {
			return nil, err
		}
		if nilAsError {
			return nil, ErrRedisNil
		}
		return nil, nil
	case '#':
		line, err := readRedisLine(reader)
		if err != nil {
			return nil, err
		}
		return line == "t", nil
	default:
		return nil, fmt.Errorf("unsupported redis reply prefix %q", prefix)
	}
}

func readRedisLine(reader *bufio.Reader) (string, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r"), nil
}

func redisBytes(value any) ([]byte, error) {
	switch v := value.(type) {
	case []byte:
		out := make([]byte, len(v))
		copy(out, v)
		return out, nil
	case string:
		return []byte(v), nil
	default:
		return nil, fmt.Errorf("redis: unexpected bulk value type %T", value)
	}
}

func redisInt64(value any) int64 {
	switch v := value.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case string:
		n, _ := strconv.ParseInt(v, 10, 64)
		return n
	case []byte:
		n, _ := strconv.ParseInt(string(v), 10, 64)
		return n
	default:
		return 0
	}
}

func redisTTLMillis(ttl time.Duration) int64 {
	ttlMillis := ttl.Milliseconds()
	if ttl > 0 && ttlMillis == 0 {
		return 1
	}
	return ttlMillis
}

func contextOrBackground(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

type redisCommandError string

func (err redisCommandError) Error() string {
	return string(err)
}

func isRedisCommandError(err error) bool {
	var commandErr redisCommandError
	return errors.As(err, &commandErr)
}

func Key(prefix, key string) string {
	prefix = strings.TrimRight(prefix, ":")
	if prefix == "" {
		return key
	}
	return prefix + ":" + key
}
