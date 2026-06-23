package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"go-entry-task/internal/config"
	"go-entry-task/internal/protocol"
	"go-entry-task/internal/store/mysqlstore"
)

type UserStore interface {
	GetUserAuthByUsername(context.Context, string) (mysqlstore.UserAuth, error)
	GetUserProfileByID(context.Context, uint64) (mysqlstore.UserProfile, error)
	UpdateNickname(context.Context, uint64, string) (mysqlstore.UserProfile, error)
	UpdateProfilePicture(context.Context, uint64, string, string, uint64) (mysqlstore.UserProfile, error)
}

type SessionStore interface {
	SaveSession(context.Context, string, uint64, time.Duration) error
	GetSession(context.Context, string) (uint64, bool, error)
	DeleteSession(context.Context, string) error
}

type PictureCache interface {
	SaveProfilePicture(context.Context, uint64, uint64, string, []byte) error
	GetProfilePicture(context.Context, uint64, uint64) (string, []byte, bool, error)
	DeleteProfilePicture(context.Context, uint64, uint64) error
}

type Options struct {
	Sessions     SessionStore
	PictureCache PictureCache
}

type Service struct {
	store                 UserStore
	sessions              SessionStore
	pictureCache          PictureCache
	tokens                *TokenManager
	uploadDir             string
	allowedMIMEs          map[string]struct{}
	maxUploadBytes        int64
	profilePictureBaseURL string
	passwordIterations    int
	maxNicknameBytes      int
}

func New(store UserStore, cfg config.Config, options ...Options) (*Service, error) {
	tokens, err := NewTokenManager(cfg.Security.TokenSecret, cfg.Security.SessionTTL.Duration)
	if err != nil {
		return nil, err
	}
	uploadDir, err := cfg.ProfilePictureDirAbs()
	if err != nil {
		return nil, err
	}
	allowed := make(map[string]struct{}, len(cfg.Upload.AllowedMIMETypes))
	for _, mimeType := range cfg.Upload.AllowedMIMETypes {
		allowed[mimeType] = struct{}{}
	}
	var opts Options
	if len(options) > 0 {
		opts = options[0]
	}
	return &Service{
		store:                 store,
		sessions:              opts.Sessions,
		pictureCache:          opts.PictureCache,
		tokens:                tokens,
		uploadDir:             uploadDir,
		allowedMIMEs:          allowed,
		maxUploadBytes:        cfg.Upload.MaxBytes,
		profilePictureBaseURL: strings.TrimRight(cfg.HTTP.ProfilePictureBaseURL, "/"),
		passwordIterations:    cfg.Security.PasswordIterations,
		maxNicknameBytes:      191 * 4,
	}, nil
}

func (s *Service) Handle(ctx context.Context, req protocol.Request) (json.RawMessage, error) {
	switch req.Op {
	case protocol.OpLogin:
		var body protocol.LoginBody
		if err := decodeBody(req.Body, &body); err != nil {
			return nil, NewAppError("bad_request", "Invalid login body.", err)
		}
		return marshal(s.Login(ctx, body))
	case protocol.OpGetProfile:
		userID, err := s.requireUser(ctx, req.Token)
		if err != nil {
			return nil, err
		}
		return marshal(s.GetProfile(ctx, userID))
	case protocol.OpGetProfilePicture:
		userID, err := s.requireUser(ctx, req.Token)
		if err != nil {
			return nil, err
		}
		return marshal(s.GetProfilePicture(ctx, userID))
	case protocol.OpUpdateNickname:
		userID, err := s.requireUser(ctx, req.Token)
		if err != nil {
			return nil, err
		}
		var body protocol.UpdateNicknameBody
		if err := decodeBody(req.Body, &body); err != nil {
			return nil, NewAppError("bad_request", "Invalid nickname body.", err)
		}
		return marshal(s.UpdateNickname(ctx, userID, body.Nickname))
	case protocol.OpUploadProfilePicture:
		userID, err := s.requireUser(ctx, req.Token)
		if err != nil {
			return nil, err
		}
		var body protocol.UploadProfilePictureBody
		if err := decodeBody(req.Body, &body); err != nil {
			return nil, NewAppError("bad_request", "Invalid upload body.", err)
		}
		return marshal(s.UploadProfilePicture(ctx, userID, body))
	case protocol.OpLogout:
		if err := s.Logout(ctx, req.Token); err != nil {
			return nil, err
		}
		return json.RawMessage(`{}`), nil
	default:
		return nil, NewAppError("unknown_operation", "Unknown TCP operation.", ErrBadRequest)
	}
}

func (s *Service) Login(ctx context.Context, body protocol.LoginBody) (protocol.LoginResult, error) {
	username := strings.TrimSpace(body.Username)
	if username == "" || body.Password == "" {
		return protocol.LoginResult{}, NewAppError("invalid_credentials", "Invalid username or password.", ErrUnauthorized)
	}
	user, err := s.store.GetUserAuthByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, mysqlstore.ErrNotFound) {
			return protocol.LoginResult{}, NewAppError("invalid_credentials", "Invalid username or password.", ErrUnauthorized)
		}
		return protocol.LoginResult{}, err
	}
	ok, err := VerifyPassword(body.Password, user.PasswordSalt, user.PasswordHash, user.PasswordIter)
	if err != nil {
		return protocol.LoginResult{}, err
	}
	if !ok {
		return protocol.LoginResult{}, NewAppError("invalid_credentials", "Invalid username or password.", ErrUnauthorized)
	}
	token, exp, err := s.tokens.Issue(user.ID, time.Now())
	if err != nil {
		return protocol.LoginResult{}, err
	}
	if s.sessions != nil {
		ttl := time.Until(time.Unix(exp, 0))
		if err := s.sessions.SaveSession(ctx, token, user.ID, ttl); err != nil {
			return protocol.LoginResult{}, err
		}
	}
	profile, err := s.store.GetUserProfileByID(ctx, user.ID)
	if err != nil {
		return protocol.LoginResult{}, err
	}
	return protocol.LoginResult{
		Token:     token,
		ExpiresAt: exp,
		User:      s.toProtocolProfile(profile),
	}, nil
}

func (s *Service) GetProfile(ctx context.Context, userID uint64) (protocol.UserProfile, error) {
	profile, err := s.store.GetUserProfileByID(ctx, userID)
	if err != nil {
		if errors.Is(err, mysqlstore.ErrNotFound) {
			return protocol.UserProfile{}, ErrNotFound
		}
		return protocol.UserProfile{}, err
	}
	return s.toProtocolProfile(profile), nil
}

func (s *Service) UpdateNickname(ctx context.Context, userID uint64, nickname string) (protocol.UserProfile, error) {
	nickname = strings.TrimSpace(nickname)
	if nickname == "" || !utf8.ValidString(nickname) || len(nickname) > s.maxNicknameBytes {
		return protocol.UserProfile{}, NewAppError("invalid_nickname", "Nickname must be valid UTF-8 and within the configured length.", ErrBadRequest)
	}
	profile, err := s.store.UpdateNickname(ctx, userID, nickname)
	if err != nil {
		if errors.Is(err, mysqlstore.ErrNotFound) {
			return protocol.UserProfile{}, ErrNotFound
		}
		return protocol.UserProfile{}, err
	}
	return s.toProtocolProfile(profile), nil
}

func (s *Service) UploadProfilePicture(ctx context.Context, userID uint64, body protocol.UploadProfilePictureBody) (protocol.UserProfile, error) {
	data, err := base64.StdEncoding.DecodeString(body.DataBase64)
	if err != nil {
		return protocol.UserProfile{}, NewAppError("invalid_upload", "Uploaded file is not valid base64.", ErrBadRequest)
	}
	if int64(len(data)) > s.maxUploadBytes {
		return protocol.UserProfile{}, ErrPayloadTooLarge
	}
	detected := http.DetectContentType(data)
	contentType := strings.TrimSpace(strings.Split(body.ContentType, ";")[0])
	if contentType == "" {
		contentType = detected
	}
	if _, ok := s.allowedMIMEs[contentType]; !ok {
		return protocol.UserProfile{}, ErrUnsupportedType
	}
	if detected != "application/octet-stream" {
		if _, ok := s.allowedMIMEs[detected]; !ok {
			return protocol.UserProfile{}, ErrUnsupportedType
		}
	}
	ext := extensionFor(contentType, body.FileName)
	relPath := fmt.Sprintf("%d/%d%s", userID, time.Now().UnixNano(), ext)
	absPath := filepath.Join(s.uploadDir, relPath)
	oldProfile, _ := s.store.GetUserProfileByID(ctx, userID)
	if err := os.MkdirAll(filepath.Dir(absPath), 0750); err != nil {
		return protocol.UserProfile{}, err
	}
	if err := os.WriteFile(absPath, data, 0640); err != nil {
		return protocol.UserProfile{}, err
	}
	profile, err := s.store.UpdateProfilePicture(ctx, userID, relPath, contentType, uint64(len(data)))
	if err != nil {
		_ = os.Remove(absPath)
		if errors.Is(err, mysqlstore.ErrNotFound) {
			return protocol.UserProfile{}, ErrNotFound
		}
		return protocol.UserProfile{}, err
	}
	if s.pictureCache != nil {
		if oldProfile.ProfileVersion > 0 {
			_ = s.pictureCache.DeleteProfilePicture(ctx, userID, oldProfile.ProfileVersion)
		}
		_ = s.pictureCache.SaveProfilePicture(ctx, userID, profile.ProfileVersion, contentType, data)
	}
	return s.toProtocolProfile(profile), nil
}

func (s *Service) GetProfilePicture(ctx context.Context, userID uint64) (protocol.ProfilePictureResult, error) {
	profile, err := s.store.GetUserProfileByID(ctx, userID)
	if err != nil {
		if errors.Is(err, mysqlstore.ErrNotFound) {
			return protocol.ProfilePictureResult{}, ErrNotFound
		}
		return protocol.ProfilePictureResult{}, err
	}
	if !profile.ProfilePicturePath.Valid || profile.ProfilePicturePath.String == "" || profile.ProfileVersion == 0 {
		return protocol.ProfilePictureResult{}, ErrNotFound
	}
	if s.pictureCache != nil {
		if mimeType, data, ok, err := s.pictureCache.GetProfilePicture(ctx, userID, profile.ProfileVersion); err != nil {
			return protocol.ProfilePictureResult{}, err
		} else if ok {
			return protocol.ProfilePictureResult{
				ContentType: mimeType,
				DataBase64:  base64.StdEncoding.EncodeToString(data),
			}, nil
		}
	}
	absPath := filepath.Join(s.uploadDir, filepath.Clean(profile.ProfilePicturePath.String))
	data, err := os.ReadFile(absPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return protocol.ProfilePictureResult{}, ErrNotFound
		}
		return protocol.ProfilePictureResult{}, err
	}
	mimeType := "application/octet-stream"
	if profile.ProfilePictureMIME.Valid && profile.ProfilePictureMIME.String != "" {
		mimeType = profile.ProfilePictureMIME.String
	}
	if s.pictureCache != nil {
		_ = s.pictureCache.SaveProfilePicture(ctx, userID, profile.ProfileVersion, mimeType, data)
	}
	return protocol.ProfilePictureResult{
		ContentType: mimeType,
		DataBase64:  base64.StdEncoding.EncodeToString(data),
	}, nil
}

func (s *Service) Logout(ctx context.Context, token string) error {
	if strings.TrimSpace(token) == "" || s.sessions == nil {
		return nil
	}
	return s.sessions.DeleteSession(ctx, token)
}

func (s *Service) requireUser(ctx context.Context, token string) (uint64, error) {
	if strings.TrimSpace(token) == "" {
		return 0, ErrUnauthorized
	}
	userID, err := s.tokens.Verify(token, time.Now())
	if err != nil {
		return 0, err
	}
	if s.sessions == nil {
		return userID, nil
	}
	storedUserID, ok, err := s.sessions.GetSession(ctx, token)
	if err != nil {
		return 0, err
	}
	if !ok || storedUserID != userID {
		return 0, ErrUnauthorized
	}
	return userID, nil
}

func (s *Service) toProtocolProfile(profile mysqlstore.UserProfile) protocol.UserProfile {
	out := protocol.UserProfile{
		ID:       profile.ID,
		Username: profile.Username,
		Nickname: profile.Nickname,
	}
	if profile.ProfilePicturePath.Valid && profile.ProfilePicturePath.String != "" {
		sep := "?"
		if strings.Contains(s.profilePictureBaseURL, "?") {
			sep = "&"
		}
		out.ProfilePictureURL = fmt.Sprintf("%s%sv=%d", s.profilePictureBaseURL, sep, profile.ProfileVersion)
	}
	return out
}

func decodeBody(raw json.RawMessage, dst any) error {
	if len(raw) == 0 {
		return ErrBadRequest
	}
	return json.Unmarshal(raw, dst)
}

func marshal(v any, err error) (json.RawMessage, error) {
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(v)
	return data, err
}

func extensionFor(contentType, fileName string) string {
	if exts, err := mime.ExtensionsByType(contentType); err == nil && len(exts) > 0 {
		return exts[0]
	}
	ext := strings.ToLower(filepath.Ext(fileName))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif":
		return ext
	default:
		return ".bin"
	}
}
