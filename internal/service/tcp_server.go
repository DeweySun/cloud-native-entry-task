package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"sync"
	"time"

	"go-entry-task/internal/config"
	"go-entry-task/internal/protocol"
)

type TCPServer struct {
	cfg     config.TCPConfig
	service *Service
	log     *slog.Logger
	jobs    chan tcpJob
	wg      sync.WaitGroup
}

type tcpJob struct {
	req  protocol.Request
	resp chan protocol.Response
}

func NewTCPServer(cfg config.TCPConfig, svc *Service, log *slog.Logger) *TCPServer {
	if log == nil {
		log = slog.Default()
	}
	return &TCPServer{
		cfg:     cfg,
		service: svc,
		log:     log,
		jobs:    make(chan tcpJob, cfg.QueueSize),
	}
}

func (s *TCPServer) ListenAndServe(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.cfg.Addr)
	if err != nil {
		return err
	}
	defer ln.Close()

	workers := s.cfg.WorkerCount
	if workers <= 0 {
		workers = 1
	}
	for i := 0; i < workers; i++ {
		s.wg.Add(1)
		go s.worker(ctx)
	}
	defer func() {
		close(s.jobs)
		s.wg.Wait()
	}()

	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()

	s.log.Info("tcp backend listening", "addr", s.cfg.Addr, "workers", workers)
	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return nil
			}
			s.log.Warn("accept failed", "error", err)
			continue
		}
		go s.handleConn(ctx, conn)
	}
}

func (s *TCPServer) handleConn(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	for {
		if err := conn.SetReadDeadline(time.Now().Add(s.cfg.ReadTimeout.Duration)); err != nil {
			s.log.Warn("set read deadline failed", "error", err)
			return
		}
		payload, err := protocol.ReadFrame(conn, s.cfg.MaxFrameBytes)
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
				return
			}
			s.log.Debug("read frame failed", "error", err)
			return
		}
		var req protocol.Request
		if err := json.Unmarshal(payload, &req); err != nil {
			s.writeResponse(conn, protocol.Response{
				Version:   protocol.Version,
				OK:        false,
				ErrorCode: "bad_request",
				Message:   "Invalid JSON request.",
			})
			continue
		}
		respCh := make(chan protocol.Response, 1)
		select {
		case s.jobs <- tcpJob{req: req, resp: respCh}:
		case <-ctx.Done():
			return
		default:
			s.writeResponse(conn, protocol.Response{
				Version:   protocol.Version,
				RequestID: req.RequestID,
				OK:        false,
				ErrorCode: "server_busy",
				Message:   "Server is busy.",
			})
			continue
		}
		select {
		case resp := <-respCh:
			s.writeResponse(conn, resp)
		case <-ctx.Done():
			return
		}
	}
}

func (s *TCPServer) worker(ctx context.Context) {
	defer s.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case job, ok := <-s.jobs:
			if !ok {
				return
			}
			body, err := s.service.Handle(ctx, job.req)
			resp := protocol.Response{
				Version:   protocol.Version,
				RequestID: job.req.RequestID,
				OK:        err == nil,
				Body:      body,
			}
			if err != nil {
				resp.ErrorCode, resp.Message = ErrorCode(err)
				s.log.Debug("request failed", "op", job.req.Op, "code", resp.ErrorCode, "error", err)
			}
			select {
			case job.resp <- resp:
			case <-ctx.Done():
				return
			}
		}
	}
}

func (s *TCPServer) writeResponse(conn net.Conn, resp protocol.Response) {
	data, err := json.Marshal(resp)
	if err != nil {
		s.log.Error("marshal response failed", "error", err)
		return
	}
	_ = conn.SetWriteDeadline(time.Now().Add(s.cfg.WriteTimeout.Duration))
	if err := protocol.WriteFrame(conn, data, s.cfg.MaxFrameBytes); err != nil {
		s.log.Debug("write response failed", "error", err)
	}
}
