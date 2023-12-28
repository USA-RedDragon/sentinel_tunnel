package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"

	"github.com/USA-RedDragon/sentinel_tunnel/internal/sentinel"
)

const (
	expectedArgs = 2
)

// https://goreleaser.com/cookbooks/using-main.version/
//
//nolint:golint,gochecknoglobals
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"

	infoLog = log.New(os.Stdout,
		"INFO: ",
		log.Ldate|log.Ltime)
	errorLog = log.New(os.Stderr,
		"ERROR: ",
		log.Ldate|log.Ltime)
	fatalLog = log.New(os.Stderr,
		"FATAL: ",
		log.Ldate|log.Ltime)
)

type SentinelTunnellingDbConfig struct {
	Name      string
	LocalPort string
}

type SentinelTunnellingConfiguration struct {
	SentinelsAddressesList []string
	Databases              []SentinelTunnellingDbConfig
}

type SentinelTunnellingClient struct {
	configuration      SentinelTunnellingConfiguration
	sentinelConnection *sentinel.Connection
}

type GetDBAddressByNameFunction func(dbName string) (string, error)

func NewSentinelTunnellingClient(configPath string) *SentinelTunnellingClient {
	data, err := os.ReadFile(configPath)
	if err != nil {
		fatalLog.Printf("an error has occur during configuration read: %v\n", err.Error())
		os.Exit(1)
	}

	tunnellingClient := SentinelTunnellingClient{}
	err = json.Unmarshal(data, &(tunnellingClient.configuration))
	if err != nil {
		fatalLog.Printf("an error has occur during configuration unmarshal: %v\n", err.Error())
		os.Exit(1)
	}

	tunnellingClient.sentinelConnection, err =
		sentinel.NewConnection(tunnellingClient.configuration.SentinelsAddressesList)
	if err != nil {
		fatalLog.Printf("an error has occur during sentinel connection creation: %v\n", err.Error())
		os.Exit(1)
	}

	infoLog.Println("done initializing tunnelling")

	return &tunnellingClient
}

func createTunnelling(conn1 net.Conn, conn2 net.Conn) {
	io.Copy(conn1, conn2)
	conn1.Close()
	conn2.Close()
}

func handleConnection(c net.Conn, dbName string,
	getDBAddressByName GetDBAddressByNameFunction) {
	dbAddress, err := getDBAddressByName(dbName)
	if err != nil {
		errorLog.Printf("cannot get db address for %s: %v\n", dbName, err.Error())
		c.Close()
		return
	}
	dbConn, err := net.Dial("tcp", dbAddress)
	if err != nil {
		errorLog.Printf("cannot connect to db %s: %v\n", dbName, err.Error())
		c.Close()
		return
	}
	go createTunnelling(c, dbConn)
	go createTunnelling(dbConn, c)
}

func handleSingleDbConnections(listeningPort string, dbName string,
	getDBAddressByName GetDBAddressByNameFunction) {

	listener, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%s", listeningPort))
	if err != nil {
		fatalLog.Printf("cannot listen to port %s: %v\n", listeningPort, err.Error())
		os.Exit(1)
	}

	infoLog.Printf("listening on port %s for connections to database: %s\n", listeningPort, dbName)
	for {
		conn, err := listener.Accept()
		if err != nil {
			fatalLog.Printf("cannot accept connections on port %s: %v\n", listeningPort, err.Error())
			os.Exit(1)
		}
		go handleConnection(conn, dbName, getDBAddressByName)
	}

}

func (stClient *SentinelTunnellingClient) Start() {
	for _, dbConf := range stClient.configuration.Databases {
		go handleSingleDbConnections(dbConf.LocalPort, dbConf.Name,
			stClient.sentinelConnection.GetAddressByDbName)
	}
}

func main() {
	infoLog.Printf("Redis Sentinel Tunnel %s (%s built %s)\n", version, string([]byte(commit)[:7]), date)
	if len(os.Args) < expectedArgs {
		fatalLog.Printf("not enough arguments\n")
		fatalLog.Printf("usage: %s <config_file_path>\n", os.Args[0])
		return
	}
	stClient := NewSentinelTunnellingClient(os.Args[1])
	stClient.Start()
	for {
		select {}
	}
}
