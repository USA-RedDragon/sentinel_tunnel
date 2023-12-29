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

type GetMasterAddrReply struct {
	reply string
	err   error
}

type Connection struct {
	sentinelsAddresses          []string
	currentConnection           net.Conn
	reader                      *bufio.Reader
	writer                      *bufio.Writer
	getMasterAddressByNameReply chan *GetMasterAddrReply
	getMasterAddressByName      chan string
}

const (
	clientClosed    = true
	clientNotClosed = false
	dialTimeout     = 300 * time.Millisecond
)

var (
	ErrReadFailed      = errors.New("failed read line from client")
	ErrWriteFailed     = errors.New("failed write to client")
	ErrInvalidResponse = errors.New("invalid response")
	ErrNullRequest     = errors.New("null request")
	ErrWrongBulkSize   = errors.New("bulk string size did not match header")
	ErrDbName          = errors.New("failed to retrieve db name from the sentinel")
	ErrConnect         = errors.New("failed to connect to any of the sentinel services")
)

func (c *Connection) parseResponse() ([]string, bool, error) {
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

func (c *Connection) retrieveAddressByDbName(password string) {
	for dbName := range c.getMasterAddressByName {
		response, isClientClosed, err := c.getMasterAddrByNameFromSentinel(dbName, password)
		if err != nil {
			slog.Error("failed to get master", "error", err.Error())
			if !isClientClosed {
				var reply string
				if len(response) != 0 {
					reply = response[0]
				}
				c.getMasterAddressByNameReply <- &GetMasterAddrReply{
					reply: reply,
					err:   fmt.Errorf("%w: %s", ErrDbName, dbName),
				}
			}
			if !c.reconnectToSentinel() {
				c.getMasterAddressByNameReply <- &GetMasterAddrReply{
					reply: "",
					err:   ErrConnect,
				}
			}
			continue
		}
		c.getMasterAddressByNameReply <- &GetMasterAddrReply{
			reply: net.JoinHostPort(response[0], response[1]),
			err:   nil,
		}
	}
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

func (c *Connection) GetAddressByDbName(name string) (string, error) {
	c.getMasterAddressByName <- name
	reply := <-c.getMasterAddressByNameReply
	return reply.reply, reply.err
}

func NewConnection(addresses []string, password string) (*Connection, error) {
	connection := Connection{
		sentinelsAddresses:          addresses,
		getMasterAddressByName:      make(chan string),
		getMasterAddressByNameReply: make(chan *GetMasterAddrReply),
		currentConnection:           nil,
		reader:                      nil,
		writer:                      nil,
	}

	if !connection.reconnectToSentinel() {
		return nil, ErrConnect
	}

	go connection.retrieveAddressByDbName(password)

	return &connection, nil
}
