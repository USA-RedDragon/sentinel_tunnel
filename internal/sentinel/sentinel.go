package sentinel

import (
	"bufio"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"time"

	"github.com/USA-RedDragon/sentinel_tunnel/internal/sentinel/resp"
	"github.com/USA-RedDragon/sentinel_tunnel/internal/sentinel/resp/token"
)

type Connection struct {
	sentinelsAddresses []string
	password           string
	retryBackoff       time.Duration
	retryCount         uint
	currentConnection  net.Conn
	reader             *bufio.Reader
	writer             *bufio.Writer
}

const (
	clientClosed    = true
	clientNotClosed = false
	dialTimeout     = 300 * time.Millisecond
)

var (
	ErrNotConnected    = errors.New("not connected to Sentinel")
	ErrReadFailed      = errors.New("failed read line from client")
	ErrWriteFailed     = errors.New("failed write to client")
	ErrInvalidResponse = errors.New("invalid response")
	ErrNullRequest     = errors.New("null request")
	ErrWrongBulkSize   = errors.New("bulk string size did not match header")
	ErrDbName          = errors.New("failed to retrieve db name from the sentinel")
	ErrConnect         = errors.New("failed to connect to any of the sentinel services")
)

func (c *Connection) parseResponse() ([]string, bool, error) {
	if c.reader == nil {
		return []string{}, clientClosed, ErrNotConnected
	}

	for {
		buf, _, e := c.reader.ReadLine()
		if e != nil || len(buf) == 0 {
			return nil, clientClosed, ErrReadFailed
		}

		switch buf[0] {
		case token.SimpleString:
			switch string(buf[1:]) {
			case "OK":
				continue
			default:
				return nil, clientNotClosed, fmt.Errorf("%w: unexpected string: %q", ErrInvalidResponse, string(buf))
			}
		case token.Array:
			arrayLen, err := strconv.Atoi(string(buf[1:]))
			if err != nil || arrayLen == -1 {
				return nil, clientNotClosed, ErrNullRequest
			}

			result := make([]string, 0, arrayLen)
			for i := 0; i < arrayLen; i++ {
				buf, _, err := c.reader.ReadLine()
				if err != nil || len(buf) == 0 {
					return nil, clientClosed, ErrReadFailed
				}
				if buf[0] != token.BulkString {
					return nil, clientNotClosed, fmt.Errorf("%w: expected bulk string header: %q", ErrInvalidResponse, string(buf))
				}

				bulkSize, err := strconv.Atoi(string(buf[1:]))
				if err != nil {
					return nil, clientNotClosed, ErrNullRequest
				}

				buf, _, err = c.reader.ReadLine()
				if err != nil {
					return nil, clientClosed, ErrReadFailed
				}

				bulk := string(buf)
				if len(bulk) != bulkSize {
					return nil, clientNotClosed, ErrWrongBulkSize
				}

				result = append(result, bulk)
			}
			return result, clientNotClosed, nil
		case token.SimpleError:
			return []string{string(buf[1:])}, clientNotClosed, fmt.Errorf("%w: got error: %q", ErrInvalidResponse, string(buf))
		default:
			return nil, clientNotClosed, fmt.Errorf("%w: expected array header: %q", ErrInvalidResponse, string(buf))
		}
	}
}

func (c *Connection) getMasterAddrByNameFromSentinel(dbName, password string) ([]string, bool, error) {
	if c.writer == nil {
		return []string{}, clientClosed, ErrNotConnected
	}

	var cmd resp.Command
	if password != "" {
		cmd = append(cmd, resp.Array{
			resp.BulkString("auth"),
			resp.BulkString(password),
		})
	}

	cmd = append(cmd, resp.Array{
		resp.BulkString("sentinel"),
		resp.BulkString("get-master-addr-by-name"),
		resp.BulkString(dbName),
	})

	_, err := c.writer.WriteString(cmd.String())
	if err != nil {
		return []string{}, clientClosed, fmt.Errorf("%w: %w", ErrWriteFailed, err)
	}

	if err := c.writer.Flush(); err != nil {
		return []string{}, clientClosed, fmt.Errorf("%w: %w", ErrWriteFailed, err)
	}

	return c.parseResponse()
}

func (c *Connection) GetAddressByDbName(dbName string) (string, error) {
	return c.getAddressByDbName(dbName, 0)
}

func (c *Connection) getAddressByDbName(dbName string, count uint) (string, error) {
	response, isClientClosed, err := c.getMasterAddrByNameFromSentinel(dbName, c.password)
	if err != nil {
		slog.Error("failed to get master", "error", err.Error())
		switch {
		case isClientClosed:
			if count < c.retryCount && c.reconnectToSentinel() {
				time.Sleep(time.Duration(count) * c.retryBackoff)
				return c.getAddressByDbName(dbName, count+1)
			}
			return "", ErrConnect
		case len(response) != 0:
			return response[0], fmt.Errorf("%w: %s", ErrDbName, dbName)
		default:
			return "", fmt.Errorf("%w: %s", ErrDbName, dbName)
		}
	}
	return net.JoinHostPort(response[0], response[1]), nil
}

func (c *Connection) reconnectToSentinel() bool {
	for _, sentinelAddr := range c.sentinelsAddresses {
		if c.currentConnection != nil {
			c.currentConnection.Close()
			c.reader = nil
			c.writer = nil
			c.currentConnection = nil
		}

		var err error
		c.currentConnection, err = net.DialTimeout("tcp", sentinelAddr, dialTimeout)
		if err == nil {
			c.reader = bufio.NewReader(c.currentConnection)
			c.writer = bufio.NewWriter(c.currentConnection)
			return true
		}
		slog.Error("failed to reconnect to Sentinel", "error", err.Error())
	}
	return false
}

func NewConnection(config TunnellingConfiguration) (*Connection, error) {
	connection := Connection{
		sentinelsAddresses: config.SentinelsAddressesList,
		password:           config.Password,
		retryBackoff:       config.RetryBackoff,
		retryCount:         config.RetryCount,
		currentConnection:  nil,
		reader:             nil,
		writer:             nil,
	}

	if !connection.reconnectToSentinel() {
		return nil, ErrConnect
	}

	return &connection, nil
}
