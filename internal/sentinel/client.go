package sentinel

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"

	"github.com/USA-RedDragon/sentinel_tunnel/internal/sentinel/resp"
	"github.com/ghodss/yaml"
	"golang.org/x/sync/errgroup"
)

type TunnellingDbConfig struct {
	Name      string
	LocalPort string
}

type TunnellingConfiguration struct {
	SentinelsAddressesList []string
	Password               string
	Databases              []TunnellingDbConfig
}

type TunnellingClient struct {
	configuration      TunnellingConfiguration
	sentinelConnection *Connection
}

type GetDBAddressByNameFunction func(dbName string) (string, error)

func NewTunnellingClient(configPath string) (*TunnellingClient, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("an error has occur during configuration read: %w", err)
	}

	tunnellingClient := TunnellingClient{}
	err = yaml.Unmarshal(data, &(tunnellingClient.configuration))
	if err != nil {
		return nil, fmt.Errorf("an error has occur during configuration unmarshal: %w", err)
	}

	tunnellingClient.sentinelConnection, err =
		NewConnection(tunnellingClient.configuration.SentinelsAddressesList, tunnellingClient.configuration.Password)
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
	response, err := getDBAddressByName(dbName)
	if err != nil {
		slog.Error("cannot get db address", "db", dbName, "error", err.Error())
		_, _ = c.Write([]byte(resp.SimpleError("ERR Tunnel failed to get db: " + response).String()))
		c.Close()
		return
	}
	dbConn, err := net.Dial("tcp", response)
	if err != nil {
		slog.Error("cannot connect to db", "db", dbName, "error", err.Error())
		_, _ = c.Write([]byte(resp.SimpleError("ERR Tunnel failed to connect: " + err.Error()).String()))
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

func (stClient *TunnellingClient) ListenAndServe(ctx context.Context) error {
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
