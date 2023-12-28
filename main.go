package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"

	"github.com/USA-RedDragon/sentinel_tunnel/internal/sentinel"
	"golang.org/x/sync/errgroup"
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

func NewSentinelTunnellingClient(configPath string) (*SentinelTunnellingClient, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("an error has occur during configuration read: %w", err)
	}

	tunnellingClient := SentinelTunnellingClient{}
	err = json.Unmarshal(data, &(tunnellingClient.configuration))
	if err != nil {
		return nil, fmt.Errorf("an error has occur during configuration unmarshal: %w", err)
	}

	tunnellingClient.sentinelConnection, err =
		sentinel.NewConnection(tunnellingClient.configuration.SentinelsAddressesList)
	if err != nil {
		return nil, fmt.Errorf("an error has occur during sentinel connection creation: %w", err)
	}

	slog.Info("done initializing tunnelling")

	return &tunnellingClient, nil
}

func createTunnelling(conn1 net.Conn, conn2 net.Conn) error {
	_, err := io.Copy(conn1, conn2)
	return errors.Join(err, conn1.Close(), conn2.Close())
}

func handleConnection(c net.Conn, dbName string,
	getDBAddressByName GetDBAddressByNameFunction) {
	dbAddress, err := getDBAddressByName(dbName)
	if err != nil {
		slog.Error("cannot get db address", "db", dbName, "error", err.Error())
		c.Close()
		return
	}
	dbConn, err := net.Dial("tcp", dbAddress)
	if err != nil {
		slog.Error("cannot connect to db", "db", dbName, "error", err.Error())
		c.Close()
		return
	}
	go func() {
		_ = createTunnelling(c, dbConn)
	}()
	go func() {
		_ = createTunnelling(dbConn, c)
	}()
}

func handleSingleDbConnections(ctx context.Context, listeningPort string, dbName string,
	getDBAddressByName GetDBAddressByNameFunction) error {
	listener, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%s", listeningPort))
	if err != nil {
		return fmt.Errorf("cannot listen to port %s: %w", listeningPort, err)
	}
	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()

	slog.Info("listening for connections to database", "port", listeningPort, "db", dbName)

	for {
		conn, err := listener.Accept()
		if err != nil {
			return fmt.Errorf("cannot accept connections on port %s: %w", listeningPort, err)
		}
		go handleConnection(conn, dbName, getDBAddressByName)
	}
}

func (stClient *SentinelTunnellingClient) ListenAndServe(ctx context.Context) error {
	group, ctx := errgroup.WithContext(ctx)
	for _, dbConf := range stClient.configuration.Databases {
		dbConf := dbConf
		group.Go(func() error {
			return handleSingleDbConnections(
				ctx,
				dbConf.LocalPort,
				dbConf.Name,
				stClient.sentinelConnection.GetAddressByDbName,
			)
		})
	}
	//nolint:wrapcheck
	return group.Wait()
}

func main() {
	slog.Info("Redis Sentinel Tunnel", "version", version, "commit", commit, "date", date)
	if len(os.Args) < expectedArgs {
		slog.Error("not enough arguments")
		slog.Error("usage:" + os.Args[0] + " <config_file_path>")
		os.Exit(1)
	}
	stClient, err := NewSentinelTunnellingClient(os.Args[1])
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
	if err := stClient.ListenAndServe(context.Background()); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}
