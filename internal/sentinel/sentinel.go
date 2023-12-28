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
)

func (c *Connection) parseResponse() (request []string, err error, isClientClosed bool) {
	var ret []string
	buf, _, e := c.reader.ReadLine()
	if e != nil {
		return nil, errors.New("failed read line from client"), clientClosed
	}
	if len(buf) == 0 {
		return nil, errors.New("failed read line from client"), clientClosed
	}
	if buf[0] != '*' {
		return nil, errors.New("first char in mbulk is not *"), clientNotClosed
	}
	mbulkSize, _ := strconv.Atoi(string(buf[1:]))
	if mbulkSize == -1 {
		return nil, errors.New("null request"), clientNotClosed
	}
	ret = make([]string, mbulkSize)
	for i := 0; i < mbulkSize; i++ {
		buf1, _, e1 := c.reader.ReadLine()
		if e1 != nil {
			return nil, errors.New("failed read line from client"), clientClosed
		}
		if len(buf1) == 0 {
			return nil, errors.New("failed read line from client"), clientClosed
		}
		if buf1[0] != '$' {
			return nil, errors.New("first char in bulk is not $"), clientNotClosed
		}
		bulkSize, _ := strconv.Atoi(string(buf1[1:]))
		buf2, _, e2 := c.reader.ReadLine()
		if e2 != nil {
			return nil, errors.New("failed read line from client"), clientClosed
		}
		bulk := string(buf2)
		if len(bulk) != bulkSize {
			return nil, errors.New("wrong bulk size"), clientNotClosed
		}
		ret[i] = bulk
	}
	return ret, nil, clientNotClosed
}

func (c *Connection) getMasterAddrByNameFromSentinel(dbName string) (addr []string, returnedErr error, isClientClosed bool) {
	c.writer.WriteString("*3\r\n")
	c.writer.WriteString("$8\r\n")
	c.writer.WriteString("sentinel\r\n")
	c.writer.WriteString("$23\r\n")
	c.writer.WriteString("get-master-addr-by-name\r\n")
	c.writer.WriteString(fmt.Sprintf("$%d\r\n", len(dbName)))
	c.writer.WriteString(dbName)
	c.writer.WriteString("\r\n")
	c.writer.Flush()

	return c.parseResponse()
}

func (c *Connection) retrieveAddressByDbName() {
	for dbName := range c.getMasterAddressByName {
		addr, err, isClientClosed := c.getMasterAddrByNameFromSentinel(dbName)
		if err != nil {
			fmt.Println("err: ", err.Error())
			if !isClientClosed {
				c.getMasterAddressByNameReply <- &GetMasterAddrReply{
					reply: "",
					err:   errors.New("failed to retrieve db name from the sentinel, db_name:" + dbName),
				}
			}
			if !c.reconnectToSentinel() {
				c.getMasterAddressByNameReply <- &GetMasterAddrReply{
					reply: "",
					err:   errors.New("failed to connect to any of the sentinel services"),
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
		c.currentConnection, err = net.DialTimeout("tcp", sentinelAddr, 300*time.Millisecond)
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
		return nil, errors.New("could not connect to any sentinels")
	}

	go connection.retrieveAddressByDbName()

	return &connection, nil
}
