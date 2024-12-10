package main

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/net/websocket"
)

const (
	DefaultBufferSize   = 16 * 1024
	DefaultWriteTimeout = 15 * time.Second
)

var sharedBufferPool = sync.Pool{
	New: func() interface{} {
		buffer := make([]byte, DefaultBufferSize)
		return &buffer
	},
}

func newBufferPool(size int) *sync.Pool {
	if size == DefaultBufferSize || size <= 0 {
		return &sharedBufferPool
	}
	return &sync.Pool{
		New: func() interface{} {
			buffer := make([]byte, size)
			return &buffer
		},
	}
}

type GetTargetFunc func(req *http.Request) (string, []string, error)

type Handler struct {
	bufferPool        *sync.Pool
	wsServer          *websocket.Server
	defaultTargetAddr string
	bufferSize        int
}

type HandlerOption func(*Handler)

func WithHandlerBufferSize(size int) HandlerOption {
	return func(h *Handler) {
		h.bufferSize = size
	}
}

func checkOrigin(config *websocket.Config, req *http.Request) (err error) {
	config.Origin, err = websocket.Origin(config, req)
	if err == nil && config.Origin == nil {
		return errors.New("null origin")
	}
	return err
}

func NewHandler(targetAddr string, opts ...HandlerOption) *Handler {
	h := &Handler{
		defaultTargetAddr: targetAddr,
	}

	for _, opt := range opts {
		opt(h)
	}

	if h.bufferSize == 0 {
		h.bufferSize = DefaultBufferSize
	}
	h.bufferPool = newBufferPool(h.bufferSize)

	h.wsServer = &websocket.Server{
		Handler:   h.handleWebSocket,
		Handshake: checkOrigin,
	}

	return h
}

func (h *Handler) getBuffer() *[]byte {
	return h.bufferPool.Get().(*[]byte)
}

func (h *Handler) putBuffer(buffer *[]byte) {
	if buffer != nil {
		*buffer = (*buffer)[:cap(*buffer)]
		h.bufferPool.Put(buffer)
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	h.wsServer.ServeHTTP(w, req)
}

func (h *Handler) handleWebSocket(ws *websocket.Conn) {
	defer ws.Close()

	ws.PayloadType = websocket.BinaryFrame

	exit := make(chan struct{})
	defer close(exit)

	go func() {
		codec := websocket.Codec{
			Marshal: func(_ any) ([]byte, byte, error) {
				return nil, websocket.PingFrame, nil
			},
		}
		ticker := time.NewTicker(time.Second * 30)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				err := codec.Send(ws, nil)
				if err == nil {
					continue
				}
				_ = ws.Close()
				return
			case <-exit:
				return
			}
		}
	}()

	h.handleNetwork(ws, h.defaultTargetAddr)
}

func (h *Handler) handleNetwork(ws *websocket.Conn, addr string) {
	conn, err := dial(ws.Request().Context(), "tcp", addr)
	if err != nil {
		return
	}
	defer conn.Close()

	go func() {
		buffer := h.getBuffer()
		defer h.putBuffer(buffer)
		_, _ = CopyBufferWithWriteTimeout(conn, ws, *buffer, DefaultWriteTimeout)
	}()

	buffer := h.getBuffer()
	defer h.putBuffer(buffer)
	_, _ = CopyBufferWithWriteTimeout(ws, conn, *buffer, DefaultWriteTimeout)
}

func dial(_ context.Context, network, addr string) (net.Conn, error) {
	return net.Dial(network, addr)
}

type deadlineWriter interface {
	io.Writer
	SetWriteDeadline(time.Time) error
}

func CopyBufferWithWriteTimeout(dst deadlineWriter, src io.Reader, buf []byte, timeout time.Duration) (written int64, err error) {
	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			err = dst.SetWriteDeadline(time.Now().Add(timeout))
			if err != nil {
				break
			}
			nw, ew := dst.Write(buf[0:nr])
			if nw < 0 || nr < nw {
				nw = 0
				if ew == nil {
					ew = errors.New("invalid write result")
				}
			}
			written += int64(nw)
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}
		if er != nil {
			if er != io.EOF {
				err = er
			}
			break
		}
	}
	return written, err
}
