package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/websocket"
)

var defaultDialer = &net.Dialer{
	Timeout: time.Second * 5,
}

type ConnectAddrConfig struct {
	Addr string
}

func (c *ConnectAddrConfig) Clone() *ConnectAddrConfig {
	return &ConnectAddrConfig{
		Addr: c.Addr,
	}
}

type ConnectDialConfig struct {
	Dialer     *net.Dialer
	Host       string
	Path       string
	ServerName string
	TLS        bool
	Insecure   bool
}

type splitedConnectDialConfig struct {
	*ConnectDialConfig
	splitAddr string
	splitPort string
}

func (c *ConnectDialConfig) Clone() *ConnectDialConfig {
	clone := *c
	return &clone
}

type ConnectConfig struct {
	ConnectAddrConfig
	ConnectDialConfig
}

func (c *ConnectConfig) Clone() *ConnectConfig {
	return &ConnectConfig{
		ConnectAddrConfig: *c.ConnectAddrConfig.Clone(),
		ConnectDialConfig: *c.ConnectDialConfig.Clone(),
	}
}

type ConnectOption func(*ConnectConfig)

func WithURL(u *url.URL) ConnectOption {
	return func(c *ConnectConfig) {
		c.Addr = u.Host
		c.Path = u.Path
		switch u.Scheme {
		case "wss":
			c.TLS = true
			c.ServerName = u.Host
		default:
			c.TLS = false
		}
	}
}

func WithAddr(addr string) ConnectOption {
	return func(c *ConnectConfig) {
		c.Addr = addr
	}
}

func WithHost(host string) ConnectOption {
	return func(c *ConnectConfig) {
		c.Host = host
	}
}

func WithPath(path string) ConnectOption {
	return func(c *ConnectConfig) {
		c.Path = path
	}
}

func WithDialTLS(serverName string, insecure bool) ConnectOption {
	return func(c *ConnectConfig) {
		c.TLS = true
		c.ServerName = serverName
		c.Insecure = insecure
	}
}

func WithDialer(dialer *net.Dialer) ConnectOption {
	return func(c *ConnectConfig) {
		c.Dialer = dialer
	}
}

func Connect(ctx context.Context, opts ...ConnectOption) (net.Conn, error) {
	cfg := ConnectConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}

	return ConnectWithConfig(ctx, cfg)
}

func ConnectWithConfig(ctx context.Context, cfg ConnectConfig) (net.Conn, error) {
	dialCfg, err := generateDialConfig(cfg.Addr, cfg.ConnectDialConfig)
	if err != nil {
		return nil, err
	}

	ws, err := connect(ctx, dialCfg)
	if err != nil {
		return nil, err
	}
	ws.PayloadType = websocket.BinaryFrame
	return ws, nil
}

func generateDialConfig(addr string, cfg ConnectDialConfig) (*splitedConnectDialConfig, error) {
	if cfg.Dialer == nil {
		cfg.Dialer = defaultDialer
	}

	addr, port, err := parseAddrAndPort(addr, cfg.TLS)
	if err != nil {
		return nil, err
	}
	splitCfg := splitedConnectDialConfig{
		splitAddr:         addr,
		splitPort:         port,
		ConnectDialConfig: &cfg,
	}

	if cfg.Host == "" {
		if cfg.ServerName != "" {
			cfg.Host = cfg.ServerName
		} else {
			cfg.Host = addr
		}
	}

	if cfg.ServerName == "" {
		cfg.ServerName = cfg.Host
	}

	cfg.Path = ensureLeadingSlash(cfg.Path)

	return &splitCfg, nil
}

func parseAddrAndPort(addr string, tlsEnabled bool) (string, string, error) {
	domain, port, err := net.SplitHostPort(addr)
	if err != nil {
		if err.Error() == "missing port in address" {
			return addr, defaultPort(tlsEnabled), nil
		}
		return "", "", fmt.Errorf("failed to split host and port: %w", err)
	}
	return domain, port, nil
}

func defaultPort(tlsEnabled bool) string {
	if tlsEnabled {
		return "443"
	}
	return "80"
}

func ensureLeadingSlash(path string) string {
	if !strings.HasPrefix(path, "/") {
		return "/" + path
	}
	return path
}

func connect(ctx context.Context, cfg *splitedConnectDialConfig) (*websocket.Conn, error) {
	wsConfig, err := createWebsocketConfig(cfg.ConnectDialConfig)
	if err != nil {
		return nil, err
	}

	dialConn, err := dialWithTimeout(ctx, cfg.Dialer, cfg.splitAddr, cfg.splitPort)
	if err != nil {
		return nil, err
	}

	if cfg.TLS {
		config := &tls.Config{
			InsecureSkipVerify: cfg.Insecure,
			ServerName:         cfg.ServerName,
		}
		tlsConn := tls.Client(dialConn, config)
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			dialConn.Close()
			return nil, err
		}
		dialConn = tlsConn
	}

	ws, err := websocket.NewClient(wsConfig, dialConn)
	if err != nil {
		dialConn.Close()
		return nil, err
	}
	return ws, nil
}

func createWebsocketConfig(cfg *ConnectDialConfig) (*websocket.Config, error) {
	var server, origin string
	if cfg.TLS {
		server = fmt.Sprintf("wss://%s%s", cfg.Host, cfg.Path)
		origin = fmt.Sprintf("https://%s%s", cfg.Host, cfg.Path)
	} else {
		server = fmt.Sprintf("ws://%s%s", cfg.Host, cfg.Path)
		origin = fmt.Sprintf("http://%s%s", cfg.Host, cfg.Path)
	}
	wsConfig, err := websocket.NewConfig(server, origin)
	if err != nil {
		return nil, fmt.Errorf("failed to create websocket config: %w", err)
	}
	setReqHeader(wsConfig)
	wsConfig.Dialer = cfg.Dialer
	return wsConfig, nil
}

func setReqHeader(wsConfig *websocket.Config) {
	wsConfig.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/86.0.4240.198 Safari/537.36")
}

func dialWithTimeout(ctx context.Context, dialer *net.Dialer, addr, port string) (net.Conn, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()
	return dialer.DialContext(timeoutCtx, "tcp", fmt.Sprintf("%s:%s", addr, port))
}

type Dialer struct {
	config ConnectConfig
}

func NewDialer(options ...ConnectOption) *Dialer {
	wc := &Dialer{}
	for _, option := range options {
		option(&wc.config)
	}
	return wc
}

func (wc *Dialer) DialContext(ctx context.Context, options ...ConnectOption) (net.Conn, error) {
	cfg := wc.config.Clone()
	for _, option := range options {
		option(cfg)
	}
	return ConnectWithConfig(ctx, *cfg)
}

func (wc *Dialer) Dial(options ...ConnectOption) (net.Conn, error) {
	return wc.DialContext(context.Background(), options...)
}

func (wc *Dialer) DialTCP(options ...ConnectOption) (net.Conn, error) {
	return wc.Dial(options...)
}

func (wc *Dialer) DialContextTCP(ctx context.Context, options ...ConnectOption) (net.Conn, error) {
	return wc.DialContext(ctx, options...)
}
