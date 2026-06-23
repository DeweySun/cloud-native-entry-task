package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync/atomic"
	"time"

	"go-entry-task/internal/config"
	"go-entry-task/internal/protocol"
)

type TCPClient struct {
	addr          string
	timeout       time.Duration
	maxFrameBytes uint32
	nextID        atomic.Uint64
}

func NewTCPClient(cfg config.Config) *TCPClient {
	return &TCPClient{
		addr:          cfg.HTTP.TCPAddr,
		timeout:       cfg.HTTP.RequestTimeout.Duration,
		maxFrameBytes: cfg.TCP.MaxFrameBytes,
	}
}

func (c *TCPClient) Call(ctx context.Context, op, token string, body any, out any) error {
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining > 0 && remaining < c.timeout {
			return c.call(ctx, op, token, body, out, remaining)
		}
	}
	return c.call(ctx, op, token, body, out, c.timeout)
}

func (c *TCPClient) call(ctx context.Context, op, token string, body any, out any, timeout time.Duration) error {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", c.addr)
	if err != nil {
		return err
	}
	defer conn.Close()
	if timeout > 0 {
		_ = conn.SetDeadline(time.Now().Add(timeout))
	}
	req := protocol.Request{
		Version:   protocol.Version,
		RequestID: fmt.Sprintf("%d", c.nextID.Add(1)),
		Op:        op,
		Token:     token,
		Body:      protocol.MarshalBody(body),
	}
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	if err := protocol.WriteFrame(conn, data, c.maxFrameBytes); err != nil {
		return err
	}
	respData, err := protocol.ReadFrame(conn, c.maxFrameBytes)
	if err != nil {
		return err
	}
	var resp protocol.Response
	if err := json.Unmarshal(respData, &resp); err != nil {
		return err
	}
	if !resp.OK {
		return &RemoteError{Code: resp.ErrorCode, Message: resp.Message}
	}
	if out == nil || len(resp.Body) == 0 {
		return nil
	}
	return json.Unmarshal(resp.Body, out)
}

type RemoteError struct {
	Code    string
	Message string
}

func (e *RemoteError) Error() string {
	return e.Code + ": " + e.Message
}

func IsRemote(err error) (*RemoteError, bool) {
	var remote *RemoteError
	if errors.As(err, &remote) {
		return remote, true
	}
	return nil, false
}
