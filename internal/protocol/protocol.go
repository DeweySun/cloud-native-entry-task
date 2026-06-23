package protocol

import "encoding/json"

const Version = 1

const (
	OpLogin                = "Login"
	OpGetProfile           = "GetProfile"
	OpGetProfilePicture    = "GetProfilePicture"
	OpUpdateNickname       = "UpdateNickname"
	OpUploadProfilePicture = "UploadProfilePicture"
	OpLogout               = "Logout"
)

type Request struct {
	Version   int             `json:"version"`
	RequestID string          `json:"request_id"`
	Op        string          `json:"op"`
	Token     string          `json:"token,omitempty"`
	Body      json.RawMessage `json:"body,omitempty"`
}

type Response struct {
	Version   int             `json:"version"`
	RequestID string          `json:"request_id"`
	OK        bool            `json:"ok"`
	ErrorCode string          `json:"error_code,omitempty"`
	Message   string          `json:"message,omitempty"`
	Body      json.RawMessage `json:"body,omitempty"`
}

type LoginBody struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResult struct {
	Token     string      `json:"token"`
	ExpiresAt int64       `json:"expires_at"`
	User      UserProfile `json:"user"`
}

type UserProfile struct {
	ID                uint64 `json:"id"`
	Username          string `json:"username"`
	Nickname          string `json:"nickname"`
	ProfilePictureURL string `json:"profile_picture_url,omitempty"`
}

type UpdateNicknameBody struct {
	Nickname string `json:"nickname"`
}

type UploadProfilePictureBody struct {
	FileName    string `json:"file_name"`
	ContentType string `json:"content_type"`
	DataBase64  string `json:"data_base64"`
}

type ProfilePictureResult struct {
	ContentType string `json:"content_type"`
	DataBase64  string `json:"data_base64"`
}

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ErrorResponse struct {
	Error Error `json:"error"`
}

func MarshalBody(v any) json.RawMessage {
	if v == nil {
		return nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}
