package main

import (
	"context"
	"net"
	"net/http"
	"sync"
	"time"
)

type Server struct {
	listener          net.Listener
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

func WithListener(listener net.Listener) ServerOption {
	return func(ps *Server) {
		ps.listener = listener
	}
}

func WithListenAddr(listenAddr string) ServerOption {
	return func(ps *Server) {
		ps.listenAddr = listenAddr
	}
}

func NewServer(path string, wsHandler *Handler, opts ...ServerOption) *Server {
	ps := &Server{
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

func (ps *Server) WaitListen() error {
	<-ps.onListened
	return ps.listenErr
}

func (ps *Server) WaitShutdown() {
	<-ps.shutdowned
}

func (ps *Server) Serve() error {
	server := ps.Server()

	defer ps.closeOnListened()
	defer close(ps.shutdowned)

	return ps.listenAndServe(server)
}

func (ps *Server) getListener() (net.Listener, error) {
	if ps.listener != nil {
		return ps.listener, nil
	}
	addr := ps.listenAddr
	if addr == "" {
		addr = ":http"
	}
	return net.Listen("tcp", addr)
}

func (ps *Server) listenAndServe(server *http.Server) error {
	ln, err := ps.getListener()
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
		ps.server.RegisterOnShutdown(func() {
			ps.wsHandler.Close()
		})
	}
	return ps.server
}

func (ps *Server) Close() error {
	defer ps.closeOnListened()
	return ps.server.Close()
}

func (ps *Server) Shutdown(ctx context.Context) error {
	defer ps.closeOnListened()
	defer ps.wsHandler.Wait()
	return ps.server.Shutdown(ctx)
}
