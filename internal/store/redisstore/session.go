package redisstore

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"

	"go-entry-task/internal/config"
)

var ErrMissing = errors.New("redis key missing")

type Store struct {
	cfg config.RedisConfig
}

type pictureValue struct {
	ContentType string `json:"content_type"`
	DataBase64  string `json:"data_base64"`
}

func New(cfg config.RedisConfig) *Store {
	return &Store{cfg: cfg}
}

func (s *Store) Ping(ctx context.Context) error {
	reply, err := s.command(ctx, "PING")
	if err != nil {
		return err
	}
	if value, ok := reply.(string); ok && value == "PONG" {
		return nil
	}
	return fmt.Errorf("unexpected redis ping reply: %v", reply)
}

func (s *Store) SaveSession(ctx context.Context, token string, userID uint64, ttl time.Duration) error {
	seconds := int(ttl.Seconds())
	if seconds <= 0 {
		return errors.New("session ttl must be positive")
	}
	_, err := s.command(ctx, "SET", s.sessionKey(token), strconv.FormatUint(userID, 10), "EX", strconv.Itoa(seconds))
	return err
}

func (s *Store) GetSession(ctx context.Context, token string) (uint64, bool, error) {
	reply, err := s.command(ctx, "GET", s.sessionKey(token))
	if err != nil {
		if errors.Is(err, ErrMissing) {
			return 0, false, nil
		}
		return 0, false, err
	}
	value, ok := reply.(string)
	if !ok || value == "" {
		return 0, false, nil
	}
	userID, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, false, err
	}
	return userID, true, nil
}

func (s *Store) DeleteSession(ctx context.Context, token string) error {
	_, err := s.command(ctx, "DEL", s.sessionKey(token))
	return err
}

func (s *Store) SaveProfilePicture(ctx context.Context, userID, version uint64, mimeType string, data []byte) error {
	value, err := json.Marshal(pictureValue{
		ContentType: mimeType,
		DataBase64:  base64.StdEncoding.EncodeToString(data),
	})
	if err != nil {
		return err
	}
	_, err = s.command(ctx, "SET", s.pictureKey(userID, version), string(value))
	return err
}

func (s *Store) GetProfilePicture(ctx context.Context, userID, version uint64) (string, []byte, bool, error) {
	reply, err := s.command(ctx, "GET", s.pictureKey(userID, version))
	if err != nil {
		if errors.Is(err, ErrMissing) {
			return "", nil, false, nil
		}
		return "", nil, false, err
	}
	value, ok := reply.(string)
	if !ok || value == "" {
		return "", nil, false, nil
	}
	var cached pictureValue
	if err := json.Unmarshal([]byte(value), &cached); err != nil {
		return "", nil, false, err
	}
	data, err := base64.StdEncoding.DecodeString(cached.DataBase64)
	if err != nil {
		return "", nil, false, err
	}
	return cached.ContentType, data, true, nil
}

func (s *Store) DeleteProfilePicture(ctx context.Context, userID, version uint64) error {
	_, err := s.command(ctx, "DEL", s.pictureKey(userID, version))
	return err
}

func (s *Store) sessionKey(token string) string {
	sum := sha256.Sum256([]byte(token))
	prefix := s.cfg.KeyPrefix
	if prefix == "" {
		prefix = "go-entry-task"
	}
	return prefix + ":session:" + hex.EncodeToString(sum[:])
}

func (s *Store) pictureKey(userID, version uint64) string {
	prefix := s.cfg.KeyPrefix
	if prefix == "" {
		prefix = "go-entry-task"
	}
	return prefix + ":profile-picture:" + strconv.FormatUint(userID, 10) + ":" + strconv.FormatUint(version, 10)
}

func (s *Store) command(ctx context.Context, args ...string) (any, error) {
	if s.cfg.Addr == "" {
		return nil, errors.New("redis address is empty")
	}
	dialer := net.Dialer{Timeout: s.cfg.DialTimeout.Duration}
	conn, err := dialer.DialContext(ctx, "tcp", s.cfg.Addr)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	if s.cfg.IOTimeout.Duration > 0 {
		_ = conn.SetDeadline(time.Now().Add(s.cfg.IOTimeout.Duration))
	}
	reader := bufio.NewReader(conn)
	if s.cfg.Password != "" {
		if err := writeCommand(conn, "AUTH", s.cfg.Password); err != nil {
			return nil, err
		}
		if _, err := readReply(reader); err != nil {
			return nil, err
		}
	}
	if s.cfg.DB > 0 {
		if err := writeCommand(conn, "SELECT", strconv.Itoa(s.cfg.DB)); err != nil {
			return nil, err
		}
		if _, err := readReply(reader); err != nil {
			return nil, err
		}
	}
	if err := writeCommand(conn, args...); err != nil {
		return nil, err
	}
	return readReply(reader)
}

func writeCommand(w io.Writer, args ...string) error {
	if _, err := fmt.Fprintf(w, "*%d\r\n", len(args)); err != nil {
		return err
	}
	for _, arg := range args {
		if _, err := fmt.Fprintf(w, "$%d\r\n%s\r\n", len(arg), arg); err != nil {
			return err
		}
	}
	return nil
}

func readReply(r *bufio.Reader) (any, error) {
	prefix, err := r.ReadByte()
	if err != nil {
		return nil, err
	}
	line, err := readLine(r)
	if err != nil {
		return nil, err
	}
	switch prefix {
	case '+':
		return line, nil
	case '-':
		return nil, errors.New(line)
	case ':':
		return strconv.ParseInt(line, 10, 64)
	case '$':
		n, err := strconv.Atoi(line)
		if err != nil {
			return nil, err
		}
		if n == -1 {
			return nil, ErrMissing
		}
		data := make([]byte, n+2)
		if _, err := io.ReadFull(r, data); err != nil {
			return nil, err
		}
		return string(data[:n]), nil
	default:
		return nil, fmt.Errorf("unknown redis reply prefix %q", prefix)
	}
}

func readLine(r *bufio.Reader) (string, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r"), nil
}
