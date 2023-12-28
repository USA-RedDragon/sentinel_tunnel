package sentinel

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"
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
	ErrWrongBulkSize   = errors.New("wrong bulk size")
	ErrDbName          = errors.New("failed to retrieve db name from the sentinel")
	ErrConnect         = errors.New("failed to connect to any of the sentinel services")
)

func (c *Connection) parseResponse() ([]string, bool, error) {
	var ret []string
	buf, _, e := c.reader.ReadLine()
	if e != nil {
		return nil, clientClosed, ErrReadFailed
	}
	if len(buf) == 0 {
		return nil, clientClosed, ErrReadFailed
	}
	if buf[0] != '*' {
		return nil, clientNotClosed, fmt.Errorf("%w: %s", ErrInvalidResponse, "first char in mbulk is not *")
	}
	mbulkSize, _ := strconv.Atoi(string(buf[1:]))
	if mbulkSize == -1 {
		return nil, clientNotClosed, ErrNullRequest
	}
	ret = make([]string, mbulkSize)
	for i := 0; i < mbulkSize; i++ {
		buf1, _, e1 := c.reader.ReadLine()
		if e1 != nil {
			return nil, clientClosed, ErrReadFailed
		}
		if len(buf1) == 0 {
			return nil, clientClosed, ErrReadFailed
		}
		if buf1[0] != '$' {
			return nil, clientNotClosed, fmt.Errorf("%w: %s", ErrInvalidResponse, "first char in bulk is not $")
		}
		bulkSize, _ := strconv.Atoi(string(buf1[1:]))
		buf2, _, e2 := c.reader.ReadLine()
		if e2 != nil {
			return nil, clientClosed, ErrReadFailed
		}
		bulk := string(buf2)
		if len(bulk) != bulkSize {
			return nil, clientNotClosed, ErrWrongBulkSize
		}
		ret[i] = bulk
	}
	return ret, clientNotClosed, nil
}

func (c *Connection) getMasterAddrByNameFromSentinel(dbName string) ([]string, bool, error) {
	getMasterCmd := "*3\r\n" +
		"$8\r\n" +
		"sentinel\r\n" +
		"$23\r\n" +
		"get-master-addr-by-name\r\n" +
		"$%d\r\n" +
		"%s\r\n"

	_, err := c.writer.WriteString(fmt.Sprintf(getMasterCmd, len(dbName), dbName))
	if err != nil {
		return []string{}, clientClosed, fmt.Errorf("%w: %w", ErrWriteFailed, err)
	}

	if err := c.writer.Flush(); err != nil {
		return []string{}, clientClosed, fmt.Errorf("%w: %w", ErrWriteFailed, err)
	}

	return c.parseResponse()
}

func (c *Connection) retrieveAddressByDbName() {
	for dbName := range c.getMasterAddressByName {
		addr, isClientClosed, err := c.getMasterAddrByNameFromSentinel(dbName)
		if err != nil {
			fmt.Println("err: ", err.Error())
			if !isClientClosed {
				c.getMasterAddressByNameReply <- &GetMasterAddrReply{
					reply: "",
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
			reply: net.JoinHostPort(addr[0], addr[1]),
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
		fmt.Println(err.Error())
	}
	return false
}

func (c *Connection) GetAddressByDbName(name string) (string, error) {
	c.getMasterAddressByName <- name
	reply := <-c.getMasterAddressByNameReply
	return reply.reply, reply.err
}

func NewConnection(addresses []string) (*Connection, error) {
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

	go connection.retrieveAddressByDbName()

	return &connection, nil
}
