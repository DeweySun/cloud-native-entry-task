package service

import "errors"

var (
	ErrBadRequest      = errors.New("bad request")
	ErrUnauthorized    = errors.New("unauthorized")
	ErrNotFound        = errors.New("not found")
	ErrPayloadTooLarge = errors.New("payload too large")
	ErrUnsupportedType = errors.New("unsupported media type")
)

type AppError struct {
	Code    string
	Message string
	Err     error
}

func (e *AppError) Error() string {
	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.Err
}

func NewAppError(code, message string, err error) *AppError {
	return &AppError{Code: code, Message: message, Err: err}
}

func ErrorCode(err error) (string, string) {
	var app *AppError
	if errors.As(err, &app) {
		return app.Code, app.Message
	}
	switch {
	case errors.Is(err, ErrBadRequest):
		return "bad_request", "Invalid request."
	case errors.Is(err, ErrUnauthorized):
		return "unauthorized", "Authentication failed."
	case errors.Is(err, ErrNotFound):
		return "not_found", "Resource not found."
	case errors.Is(err, ErrPayloadTooLarge):
		return "payload_too_large", "Uploaded file is too large."
	case errors.Is(err, ErrUnsupportedType):
		return "unsupported_media_type", "Unsupported file type."
	default:
		return "internal_error", "Internal server error."
	}
}
