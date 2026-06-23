package mysqlstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

var ErrNotFound = errors.New("not found")

type Store struct {
	db *sql.DB
}

type UserAuth struct {
	ID                    uint64
	Username              string
	Nickname              string
	PasswordSalt          []byte
	PasswordHash          []byte
	PasswordIter          int
	ProfilePicturePath    sql.NullString
	ProfilePictureVersion uint64
}

type UserProfile struct {
	ID                 uint64
	Username           string
	Nickname           string
	ProfilePicturePath sql.NullString
	ProfilePictureMIME sql.NullString
	ProfilePictureSize uint64
	ProfileVersion     uint64
}

type SeedUser struct {
	Username     string
	Nickname     string
	PasswordSalt []byte
	PasswordHash []byte
	PasswordIter int
}

func Open(dsn string, maxOpen, maxIdle int, maxLifetime time.Duration) (*sql.DB, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxIdle)
	db.SetConnMaxLifetime(maxLifetime)
	return db, nil
}

func New(db *sql.DB) *Store {
	return &Store{db: db}
}

func Migrate(ctx context.Context, db *sql.DB, schemaPath string) error {
	data, err := os.ReadFile(schemaPath)
	if err != nil {
		return err
	}
	for _, stmt := range splitSQL(string(data)) {
		if strings.TrimSpace(stmt) == "" {
			continue
		}
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("exec schema statement: %w", err)
		}
	}
	return nil
}

func (s *Store) GetUserAuthByUsername(ctx context.Context, username string) (UserAuth, error) {
	const q = `
SELECT id, username, nickname, password_salt, password_hash, password_iter,
       profile_picture_path, profile_picture_version
FROM users
WHERE username = ?
LIMIT 1`
	var u UserAuth
	err := s.db.QueryRowContext(ctx, q, username).Scan(
		&u.ID,
		&u.Username,
		&u.Nickname,
		&u.PasswordSalt,
		&u.PasswordHash,
		&u.PasswordIter,
		&u.ProfilePicturePath,
		&u.ProfilePictureVersion,
	)
	return u, mapNoRows(err)
}

func (s *Store) GetUserProfileByID(ctx context.Context, id uint64) (UserProfile, error) {
	const q = `
SELECT id, username, nickname, profile_picture_path, profile_picture_mime,
       profile_picture_size, profile_picture_version
FROM users
WHERE id = ?
LIMIT 1`
	var u UserProfile
	err := s.db.QueryRowContext(ctx, q, id).Scan(
		&u.ID,
		&u.Username,
		&u.Nickname,
		&u.ProfilePicturePath,
		&u.ProfilePictureMIME,
		&u.ProfilePictureSize,
		&u.ProfileVersion,
	)
	return u, mapNoRows(err)
}

func (s *Store) UpdateNickname(ctx context.Context, id uint64, nickname string) (UserProfile, error) {
	const q = `UPDATE users SET nickname = ? WHERE id = ?`
	res, err := s.db.ExecContext(ctx, q, nickname, id)
	if err != nil {
		return UserProfile{}, err
	}
	if affected, err := res.RowsAffected(); err == nil && affected == 0 {
		return UserProfile{}, ErrNotFound
	}
	return s.GetUserProfileByID(ctx, id)
}

func (s *Store) UpdateProfilePicture(ctx context.Context, id uint64, relPath, mime string, size uint64) (UserProfile, error) {
	const q = `
UPDATE users
SET profile_picture_path = ?,
    profile_picture_mime = ?,
    profile_picture_size = ?,
    profile_picture_version = profile_picture_version + 1
WHERE id = ?`
	res, err := s.db.ExecContext(ctx, q, relPath, mime, size, id)
	if err != nil {
		return UserProfile{}, err
	}
	if affected, err := res.RowsAffected(); err == nil && affected == 0 {
		return UserProfile{}, ErrNotFound
	}
	return s.GetUserProfileByID(ctx, id)
}

func (s *Store) InsertSeedUsers(ctx context.Context, users []SeedUser) error {
	if len(users) == 0 {
		return nil
	}
	var b strings.Builder
	args := make([]any, 0, len(users)*5)
	b.WriteString(`INSERT INTO users (username, nickname, password_salt, password_hash, password_iter) VALUES `)
	for i, u := range users {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(`(?, ?, ?, ?, ?)`)
		args = append(args, u.Username, u.Nickname, u.PasswordSalt, u.PasswordHash, u.PasswordIter)
	}
	b.WriteString(` ON DUPLICATE KEY UPDATE username = username`)
	_, err := s.db.ExecContext(ctx, b.String(), args...)
	return err
}

func (s *Store) CountUsers(ctx context.Context) (uint64, error) {
	var count uint64
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&count)
	return count, err
}

func mapNoRows(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func splitSQL(sqlText string) []string {
	parts := strings.Split(sqlText, ";")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
