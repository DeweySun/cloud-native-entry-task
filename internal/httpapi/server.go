package httpapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"go-entry-task/internal/config"
	"go-entry-task/internal/protocol"
)

type Server struct {
	cfg    config.Config
	client *TCPClient
	log    *slog.Logger
	mux    *http.ServeMux
}

func NewServer(cfg config.Config, client *TCPClient, log *slog.Logger) *Server {
	if log == nil {
		log = slog.Default()
	}
	s := &Server{
		cfg:    cfg,
		client: client,
		log:    log,
		mux:    http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) ListenAndServe(ctx context.Context) error {
	server := &http.Server{
		Addr:              s.cfg.HTTP.Addr,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	s.log.Info("http gateway listening", "addr", s.cfg.HTTP.Addr)
	err := server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.health)
	s.mux.HandleFunc("POST /api/login", s.login)
	s.mux.HandleFunc("GET /api/me", s.me)
	s.mux.HandleFunc("GET /api/me/profile-picture", s.getProfilePicture)
	s.mux.HandleFunc("PUT /api/me/nickname", s.updateNickname)
	s.mux.HandleFunc("POST /api/me/profile-picture", s.uploadProfilePicture)
	s.mux.HandleFunc("POST /api/logout", s.logout)
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var body protocol.LoginBody
	if !decodeJSON(w, r, &body, s.cfg.HTTP.MaxBodyBytes) {
		return
	}
	var result protocol.LoginResult
	if err := s.client.Call(r.Context(), protocol.OpLogin, "", body, &result); err != nil {
		s.writeError(w, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     s.cfg.HTTP.CookieName,
		Value:    result.Token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(result.ExpiresAt, 0),
	})
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	token, ok := s.tokenFromCookie(w, r)
	if !ok {
		return
	}
	var profile protocol.UserProfile
	if err := s.client.Call(r.Context(), protocol.OpGetProfile, token, nil, &profile); err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

func (s *Server) updateNickname(w http.ResponseWriter, r *http.Request) {
	token, ok := s.tokenFromCookie(w, r)
	if !ok {
		return
	}
	var body protocol.UpdateNicknameBody
	if !decodeJSON(w, r, &body, s.cfg.HTTP.MaxBodyBytes) {
		return
	}
	var profile protocol.UserProfile
	if err := s.client.Call(r.Context(), protocol.OpUpdateNickname, token, body, &profile); err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

func (s *Server) getProfilePicture(w http.ResponseWriter, r *http.Request) {
	token, ok := s.tokenFromCookie(w, r)
	if !ok {
		return
	}
	var result protocol.ProfilePictureResult
	if err := s.client.Call(r.Context(), protocol.OpGetProfilePicture, token, nil, &result); err != nil {
		s.writeError(w, err)
		return
	}
	data, err := base64.StdEncoding.DecodeString(result.DataBase64)
	if err != nil {
		writeProtocolError(w, http.StatusInternalServerError, "invalid_picture_cache", "Profile picture data is invalid.")
		return
	}
	w.Header().Set("Content-Type", result.ContentType)
	w.Header().Set("Cache-Control", "private, max-age=300")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (s *Server) uploadProfilePicture(w http.ResponseWriter, r *http.Request) {
	token, ok := s.tokenFromCookie(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, s.cfg.HTTP.MaxBodyBytes)
	if err := r.ParseMultipartForm(s.cfg.HTTP.MaxBodyBytes); err != nil {
		writeProtocolError(w, http.StatusBadRequest, "bad_request", "Invalid multipart upload.")
		return
	}
	file, header, err := r.FormFile("picture")
	if err != nil {
		writeProtocolError(w, http.StatusBadRequest, "bad_request", "Missing picture file.")
		return
	}
	defer file.Close()
	body, err := s.uploadBody(file, header)
	if err != nil {
		writeProtocolError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	var profile protocol.UserProfile
	if err := s.client.Call(r.Context(), protocol.OpUploadProfilePicture, token, body, &profile); err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(s.cfg.HTTP.CookieName); err == nil && cookie.Value != "" {
		_ = s.client.Call(r.Context(), protocol.OpLogout, cookie.Value, map[string]string{}, nil)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     s.cfg.HTTP.CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
		SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
}

func (s *Server) tokenFromCookie(w http.ResponseWriter, r *http.Request) (string, bool) {
	cookie, err := r.Cookie(s.cfg.HTTP.CookieName)
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		writeProtocolError(w, http.StatusUnauthorized, "unauthorized", "Authentication required.")
		return "", false
	}
	return cookie.Value, true
}

func (s *Server) uploadBody(file multipart.File, header *multipart.FileHeader) (protocol.UploadProfilePictureBody, error) {
	data, err := io.ReadAll(io.LimitReader(file, s.cfg.HTTP.MaxBodyBytes+1))
	if err != nil {
		return protocol.UploadProfilePictureBody{}, err
	}
	if int64(len(data)) > s.cfg.HTTP.MaxBodyBytes {
		return protocol.UploadProfilePictureBody{}, errors.New("uploaded file is too large")
	}
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}
	return protocol.UploadProfilePictureBody{
		FileName:    header.Filename,
		ContentType: contentType,
		DataBase64:  base64.StdEncoding.EncodeToString(data),
	}, nil
}

func (s *Server) writeError(w http.ResponseWriter, err error) {
	if remote, ok := IsRemote(err); ok {
		status := statusFor(remote.Code)
		writeProtocolError(w, status, remote.Code, remote.Message)
		return
	}
	s.log.Warn("gateway error", "error", err)
	writeProtocolError(w, http.StatusBadGateway, "tcp_backend_unavailable", "Backend service is unavailable.")
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any, maxBytes int64) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeProtocolError(w, http.StatusBadRequest, "bad_request", "Invalid JSON body.")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeProtocolError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, protocol.ErrorResponse{
		Error: protocol.Error{Code: code, Message: message},
	})
}

func statusFor(code string) int {
	switch code {
	case "unauthorized", "invalid_credentials":
		return http.StatusUnauthorized
	case "not_found":
		return http.StatusNotFound
	case "payload_too_large":
		return http.StatusRequestEntityTooLarge
	case "unsupported_media_type":
		return http.StatusUnsupportedMediaType
	case "server_busy":
		return http.StatusTooManyRequests
	case "bad_request", "invalid_nickname", "invalid_upload", "unknown_operation":
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}
