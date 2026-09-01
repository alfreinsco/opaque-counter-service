package redisx

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
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

func (c *Client) All() (map[string]int64, error) {
	if c.closed.Load() {
		return nil, errors.New("redis client closed")
	}

	rc, err := c.get()
	if err != nil {
		return nil, err
	}

	healthy := false
	defer func() {
		if healthy {
			c.put(rc)
		} else {
			_ = rc.Close()
		}
	}()

	result := make(map[string]int64)
	cursor := "0"
	for {
		if err := rc.SetDeadline(time.Now().Add(c.cfg.IOTimeout)); err != nil {
			return nil, err
		}
		if err := writeCommand(rc, "SCAN", cursor, "MATCH", c.cfg.KeyPrefix+"*", "COUNT", "1000"); err != nil {
			return nil, err
		}

		nextCursor, keys, err := readScanResult(rc.reader)
		if err != nil {
			return nil, err
		}

		for _, key := range keys {
			if err := rc.SetDeadline(time.Now().Add(c.cfg.IOTimeout)); err != nil {
				return nil, err
			}
			if err := writeCommand(rc, "GET", key); err != nil {
				return nil, err
			}

			value, exists, err := readBulkString(rc.reader)
			if err != nil {
				return nil, err
			}
			if !exists {
				continue
			}

			count, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid counter value for %q: %w", key, err)
			}
			result[strings.TrimPrefix(key, c.cfg.KeyPrefix)] = count
		}

		cursor = nextCursor
		if cursor == "0" {
			break
		}
	}

	healthy = true
	return result, nil
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

func readScanResult(r *bufio.Reader) (string, []string, error) {
	length, err := readArrayLength(r)
	if err != nil {
		return "", nil, err
	}
	if length != 2 {
		return "", nil, errors.New("invalid redis SCAN response")
	}

	cursor, exists, err := readBulkString(r)
	if err != nil || !exists {
		return "", nil, errors.New("invalid redis SCAN cursor")
	}

	keyCount, err := readArrayLength(r)
	if err != nil {
		return "", nil, err
	}
	keys := make([]string, 0, keyCount)
	for i := 0; i < keyCount; i++ {
		key, exists, err := readBulkString(r)
		if err != nil || !exists {
			return "", nil, errors.New("invalid redis SCAN key")
		}
		keys = append(keys, key)
	}

	return cursor, keys, nil
}

func readArrayLength(r *bufio.Reader) (int, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return 0, err
	}
	if len(line) < 4 || line[0] != '*' {
		return 0, errors.New("unexpected redis array response")
	}
	length, err := strconv.Atoi(line[1 : len(line)-2])
	if err != nil {
		return 0, err
	}
	if length < 0 {
		return 0, errors.New("invalid redis array length")
	}
	return length, nil
}

func readBulkString(r *bufio.Reader) (string, bool, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return "", false, err
	}
	if len(line) < 4 {
		return "", false, errors.New("invalid redis bulk response")
	}
	if line[0] == '-' {
		return "", false, errors.New(line[1 : len(line)-2])
	}
	if line[0] != '$' {
		return "", false, errors.New("unexpected redis bulk response")
	}

	length, err := strconv.Atoi(line[1 : len(line)-2])
	if err != nil {
		return "", false, err
	}
	if length == -1 {
		return "", false, nil
	}
	if length < 0 {
		return "", false, errors.New("invalid redis bulk length")
	}

	payload := make([]byte, length+2)
	if _, err := io.ReadFull(r, payload); err != nil {
		return "", false, err
	}
	if payload[length] != '\r' || payload[length+1] != '\n' {
		return "", false, errors.New("invalid redis bulk terminator")
	}
	return string(payload[:length]), true, nil
}
