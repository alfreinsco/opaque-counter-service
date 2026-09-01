package redisx

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

type Config struct {
	Address     string
	Password    string
	DB          int
	DialTimeout time.Duration
	IOTimeout   time.Duration
	KeyPrefix   string
}

type Client struct {
	cfg    Config
	pool   chan *conn
	closed atomic.Bool
	once   sync.Once
}

type conn struct {
	net.Conn
	reader *bufio.Reader
}

func New(cfg Config) *Client {
	if cfg.DialTimeout <= 0 {
		cfg.DialTimeout = 2 * time.Second
	}
	if cfg.IOTimeout <= 0 {
		cfg.IOTimeout = 2 * time.Second
	}
	if cfg.KeyPrefix == "" {
		cfg.KeyPrefix = "c:v1:"
	}

	return &Client{
		cfg:  cfg,
		pool: make(chan *conn, 128),
	}
}

func (c *Client) Increment(token string) error {
	if c.closed.Load() {
		return errors.New("redis client closed")
	}

	rc, err := c.get()
	if err != nil {
		return err
	}

	healthy := false
	defer func() {
		if healthy {
			c.put(rc)
		} else {
			_ = rc.Close()
		}
	}()

	if err := rc.SetDeadline(time.Now().Add(c.cfg.IOTimeout)); err != nil {
		return err
	}

	if err := writeCommand(rc, "INCR", c.cfg.KeyPrefix+token); err != nil {
		return err
	}

	if _, err := readInteger(rc.reader); err != nil {
		return err
	}

	healthy = true
	return nil
}

func (c *Client) get() (*conn, error) {
	select {
	case rc := <-c.pool:
		return rc, nil
	default:
		return c.dial()
	}
}

func (c *Client) put(rc *conn) {
	if c.closed.Load() {
		_ = rc.Close()
		return
	}

	_ = rc.SetDeadline(time.Time{})

	select {
	case c.pool <- rc:
	default:
		_ = rc.Close()
	}
}

func (c *Client) dial() (*conn, error) {
	raw, err := net.DialTimeout("tcp", c.cfg.Address, c.cfg.DialTimeout)
	if err != nil {
		return nil, err
	}

	rc := &conn{Conn: raw, reader: bufio.NewReaderSize(raw, 4096)}

	if c.cfg.Password != "" {
		if err := rc.SetDeadline(time.Now().Add(c.cfg.IOTimeout)); err != nil {
			_ = rc.Close()
			return nil, err
		}
		if err := writeCommand(rc, "AUTH", c.cfg.Password); err != nil {
			_ = rc.Close()
			return nil, err
		}
		if err := readSimpleOK(rc.reader); err != nil {
			_ = rc.Close()
			return nil, err
		}
	}

	if c.cfg.DB != 0 {
		if err := rc.SetDeadline(time.Now().Add(c.cfg.IOTimeout)); err != nil {
			_ = rc.Close()
			return nil, err
		}
		if err := writeCommand(rc, "SELECT", strconv.Itoa(c.cfg.DB)); err != nil {
			_ = rc.Close()
			return nil, err
		}
		if err := readSimpleOK(rc.reader); err != nil {
			_ = rc.Close()
			return nil, err
		}
	}

	_ = rc.SetDeadline(time.Time{})
	return rc, nil
}

func (c *Client) Close() {
	c.once.Do(func() {
		c.closed.Store(true)
		close(c.pool)
		for rc := range c.pool {
			_ = rc.Close()
		}
	})
}

func writeCommand(w io.Writer, args ...string) error {
	var b bytes.Buffer
	fmt.Fprintf(&b, "*%d\r\n", len(args))
	for _, arg := range args {
		fmt.Fprintf(&b, "$%d\r\n%s\r\n", len(arg), arg)
	}
	_, err := w.Write(b.Bytes())
	return err
}

func readInteger(r *bufio.Reader) (int64, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return 0, err
	}
	if len(line) < 3 {
		return 0, errors.New("invalid redis response")
	}
	switch line[0] {
	case ':':
		return strconv.ParseInt(line[1:len(line)-2], 10, 64)
	case '-':
		return 0, errors.New(line[1 : len(line)-2])
	default:
		return 0, errors.New("unexpected redis response")
	}
}

func readSimpleOK(r *bufio.Reader) error {
	line, err := r.ReadString('\n')
	if err != nil {
		return err
	}
	if line == "+OK\r\n" {
		return nil
	}
	if len(line) >= 3 && line[0] == '-' {
		return errors.New(line[1 : len(line)-2])
	}
	return errors.New("unexpected redis response")
}
