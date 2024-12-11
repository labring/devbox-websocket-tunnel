package main

import (
	"context"
	"net"
	"net/http"
	"sync"
	"time"
)

type Server struct {
	listenErr         error
	shutdowned        chan struct{}
	onListened        chan struct{}
	server            *http.Server
	wsHandler         *Handler
	path              string
	listenAddr        string
	onListenCloseOnce sync.Once
}

type ServerOption func(*Server)

func NewServer(listenAddr, path string, wsHandler *Handler, opts ...ServerOption) *Server {
	ps := &Server{
		listenAddr: listenAddr,
		wsHandler:  wsHandler,
		path:       path,
		onListened: make(chan struct{}),
		shutdowned: make(chan struct{}),
	}

	for _, opt := range opts {
		opt(ps)
	}

	return ps
}

func (ps *Server) closeOnListened() {
	ps.onListenCloseOnce.Do(func() {
		close(ps.onListened)
	})
}

func (ps *Server) OnListened() <-chan struct{} {
	return ps.onListened
}

func (ps *Server) ListenErr() error {
	return ps.listenErr
}

func (ps *Server) Shutdowned() <-chan struct{} {
	return ps.shutdowned
}

func (ps *Server) ShutdownedBool() bool {
	select {
	case <-ps.shutdowned:
		return true
	default:
		return false
	}
}

func (ps *Server) Serve() error {
	server := ps.Server()

	defer ps.closeOnListened()
	defer close(ps.shutdowned)

	return ps.listenAndServe(server)
}

func (ps *Server) listenAndServe(server *http.Server) error {
	addr := ps.listenAddr
	if addr == "" {
		addr = ":http"
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		ps.listenErr = err
		return err
	}
	defer ln.Close()

	ps.closeOnListened()

	return server.Serve(ln)
}

func (ps *Server) Server() *http.Server {
	if ps.server == nil {
		mux := http.NewServeMux()
		mux.Handle(ps.path, ps.wsHandler)
		ps.server = &http.Server{
			Addr:              ps.listenAddr,
			Handler:           mux,
			ReadHeaderTimeout: time.Second * 5,
			MaxHeaderBytes:    16 * 1024,
		}
	}
	return ps.server
}

func (ps *Server) Close() error {
	timeoutCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return ps.Shutdown(timeoutCtx)
}

func (ps *Server) Shutdown(ctx context.Context) error {
	ps.closeOnListened()
	return ps.server.Shutdown(ctx)
}
