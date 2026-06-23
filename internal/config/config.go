package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Duration struct {
	time.Duration
}

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

func (d *Duration) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	d.Duration = parsed
	return nil
}

type Config struct {
	DBCP     DBCPConfig     `json:"dbcp"`
	TCP      TCPConfig      `json:"tcp"`
	HTTP     HTTPConfig     `json:"http"`
	Database DatabaseConfig `json:"database"`
	Redis    RedisConfig    `json:"redis"`
	Security SecurityConfig `json:"security"`
	Upload   UploadConfig   `json:"upload"`
	Seed     SeedConfig     `json:"seed"`
}

type DBCPConfig struct {
	TargetDB          string `json:"target_db"`
	TargetRedis       string `json:"target_redis"`
	ServiceExportPort int    `json:"service_export_port"`
}

type TCPConfig struct {
	Addr          string   `json:"addr"`
	ReadTimeout   Duration `json:"read_timeout"`
	WriteTimeout  Duration `json:"write_timeout"`
	MaxFrameBytes uint32   `json:"max_frame_bytes"`
	WorkerCount   int      `json:"worker_count"`
	QueueSize     int      `json:"queue_size"`
}

type HTTPConfig struct {
	Addr                  string   `json:"addr"`
	TCPAddr               string   `json:"tcp_addr"`
	RequestTimeout        Duration `json:"request_timeout"`
	MaxBodyBytes          int64    `json:"max_body_bytes"`
	CookieName            string   `json:"cookie_name"`
	ProfilePictureBaseURL string   `json:"profile_picture_base_url"`
}

type DatabaseConfig struct {
	DSN             string   `json:"dsn"`
	MaxOpenConns    int      `json:"max_open_conns"`
	MaxIdleConns    int      `json:"max_idle_conns"`
	ConnMaxLifetime Duration `json:"conn_max_lifetime"`
}

type RedisConfig struct {
	Addr        string   `json:"addr"`
	Password    string   `json:"password"`
	DB          int      `json:"db"`
	KeyPrefix   string   `json:"key_prefix"`
	DialTimeout Duration `json:"dial_timeout"`
	IOTimeout   Duration `json:"io_timeout"`
}

type SecurityConfig struct {
	TokenSecret        string   `json:"token_secret"`
	PasswordIterations int      `json:"password_iterations"`
	SessionTTL         Duration `json:"session_ttl"`
}

type UploadConfig struct {
	ProfilePictureDir string   `json:"profile_picture_dir"`
	MaxBytes          int64    `json:"max_bytes"`
	AllowedMIMETypes  []string `json:"allowed_mime_types"`
}

type SeedConfig struct {
	Count           int    `json:"count"`
	BatchSize       int    `json:"batch_size"`
	Concurrency     int    `json:"concurrency"`
	DefaultPassword string `json:"default_password"`
}

func Default() Config {
	return Config{
		TCP: TCPConfig{
			Addr:          "127.0.0.1:9000",
			ReadTimeout:   Duration{5 * time.Second},
			WriteTimeout:  Duration{5 * time.Second},
			MaxFrameBytes: 8 << 20,
			WorkerCount:   64,
			QueueSize:     2048,
		},
		HTTP: HTTPConfig{
			Addr:                  "127.0.0.1:8081",
			TCPAddr:               "127.0.0.1:9000",
			RequestTimeout:        Duration{5 * time.Second},
			MaxBodyBytes:          4 << 20,
			CookieName:            "session_token",
			ProfilePictureBaseURL: "/profile-pictures/",
		},
		Database: DatabaseConfig{
			MaxOpenConns:    128,
			MaxIdleConns:    32,
			ConnMaxLifetime: Duration{5 * time.Minute},
		},
		Redis: RedisConfig{
			KeyPrefix:   "go-entry-task",
			DialTimeout: Duration{2 * time.Second},
			IOTimeout:   Duration{2 * time.Second},
		},
		Security: SecurityConfig{
			PasswordIterations: 210000,
			SessionTTL:         Duration{24 * time.Hour},
		},
		Upload: UploadConfig{
			ProfilePictureDir: "runtime/profile-pictures",
			MaxBytes:          2 << 20,
			AllowedMIMETypes:  []string{"image/jpeg", "image/png", "image/webp", "image/gif"},
		},
		Seed: SeedConfig{
			Count:           10000000,
			BatchSize:       1000,
			Concurrency:     4,
			DefaultPassword: "Password123!",
		},
	}
}

func Load(path string) (Config, error) {
	cfg := Default()
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return Config{}, err
		}
		if err := json.Unmarshal(data, &cfg); err != nil {
			return Config{}, err
		}
	}
	applyDBCP(&cfg)
	applyEnv(&cfg)
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	var missing []string
	if c.TCP.Addr == "" {
		missing = append(missing, "tcp.addr")
	}
	if c.HTTP.Addr == "" {
		missing = append(missing, "http.addr")
	}
	if c.HTTP.TCPAddr == "" {
		missing = append(missing, "http.tcp_addr")
	}
	if c.Database.DSN == "" {
		missing = append(missing, "database.dsn")
	}
	if c.Security.TokenSecret == "" {
		missing = append(missing, "security.token_secret")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required config: %s", strings.Join(missing, ", "))
	}
	if c.TCP.MaxFrameBytes == 0 {
		return errors.New("tcp.max_frame_bytes must be greater than zero")
	}
	if c.Upload.MaxBytes <= 0 {
		return errors.New("upload.max_bytes must be greater than zero")
	}
	return nil
}

func (c Config) ProfilePictureDirAbs() (string, error) {
	if filepath.IsAbs(c.Upload.ProfilePictureDir) {
		return c.Upload.ProfilePictureDir, nil
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(wd, c.Upload.ProfilePictureDir), nil
}

func applyEnv(c *Config) {
	setString(&c.DBCP.TargetDB, "DBCP_TARGET_DB")
	setString(&c.DBCP.TargetRedis, "DBCP_TARGET_REDIS")
	setInt(&c.DBCP.ServiceExportPort, "DBCP_SERVICE_EXPORT_PORT")
	setString(&c.TCP.Addr, "APP_TCP_ADDR")
	setString(&c.HTTP.Addr, "APP_HTTP_ADDR")
	setString(&c.HTTP.TCPAddr, "APP_HTTP_TCP_ADDR")
	setString(&c.HTTP.CookieName, "APP_COOKIE_NAME")
	setString(&c.HTTP.ProfilePictureBaseURL, "APP_PROFILE_PICTURE_BASE_URL")
	setString(&c.Database.DSN, "APP_DB_DSN")
	setString(&c.Redis.Addr, "APP_REDIS_ADDR")
	setString(&c.Redis.Password, "APP_REDIS_PASSWORD")
	setString(&c.Redis.KeyPrefix, "APP_REDIS_KEY_PREFIX")
	setString(&c.Security.TokenSecret, "APP_TOKEN_SECRET")
	setString(&c.Upload.ProfilePictureDir, "APP_PROFILE_PICTURE_DIR")

	setInt(&c.TCP.WorkerCount, "APP_TCP_WORKERS")
	setInt(&c.TCP.QueueSize, "APP_TCP_QUEUE_SIZE")
	setUint32(&c.TCP.MaxFrameBytes, "APP_TCP_MAX_FRAME_BYTES")
	setInt64(&c.HTTP.MaxBodyBytes, "APP_HTTP_MAX_BODY_BYTES")
	setInt64(&c.Upload.MaxBytes, "APP_UPLOAD_MAX_BYTES")
	setInt(&c.Database.MaxOpenConns, "APP_DB_MAX_OPEN_CONNS")
	setInt(&c.Database.MaxIdleConns, "APP_DB_MAX_IDLE_CONNS")
	setInt(&c.Redis.DB, "APP_REDIS_DB")
	setInt(&c.Security.PasswordIterations, "APP_PASSWORD_ITERATIONS")
	setInt(&c.Seed.Count, "APP_SEED_COUNT")
	setInt(&c.Seed.BatchSize, "APP_SEED_BATCH_SIZE")
	setInt(&c.Seed.Concurrency, "APP_SEED_CONCURRENCY")
	setString(&c.Seed.DefaultPassword, "APP_SEED_DEFAULT_PASSWORD")

	setDuration(&c.TCP.ReadTimeout, "APP_TCP_READ_TIMEOUT")
	setDuration(&c.TCP.WriteTimeout, "APP_TCP_WRITE_TIMEOUT")
	setDuration(&c.HTTP.RequestTimeout, "APP_HTTP_REQUEST_TIMEOUT")
	setDuration(&c.Database.ConnMaxLifetime, "APP_DB_CONN_MAX_LIFETIME")
	setDuration(&c.Redis.DialTimeout, "APP_REDIS_DIAL_TIMEOUT")
	setDuration(&c.Redis.IOTimeout, "APP_REDIS_IO_TIMEOUT")
	setDuration(&c.Security.SessionTTL, "APP_SESSION_TTL")

	applyDBCP(c)
}

func applyDBCP(c *Config) {
	if c.DBCP.TargetDB != "" {
		c.Database.DSN = c.DBCP.TargetDB
	}
	if c.DBCP.TargetRedis != "" {
		c.Redis.Addr = c.DBCP.TargetRedis
	}
}

func setString(dst *string, key string) {
	if v := os.Getenv(key); v != "" {
		*dst = v
	}
}

func setInt(dst *int, key string) {
	if v := os.Getenv(key); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			*dst = parsed
		}
	}
}

func setInt64(dst *int64, key string) {
	if v := os.Getenv(key); v != "" {
		if parsed, err := strconv.ParseInt(v, 10, 64); err == nil {
			*dst = parsed
		}
	}
}

func setUint32(dst *uint32, key string) {
	if v := os.Getenv(key); v != "" {
		if parsed, err := strconv.ParseUint(v, 10, 32); err == nil {
			*dst = uint32(parsed)
		}
	}
}

func setDuration(dst *Duration, key string) {
	if v := os.Getenv(key); v != "" {
		if parsed, err := time.ParseDuration(v); err == nil {
			dst.Duration = parsed
		}
	}
}
