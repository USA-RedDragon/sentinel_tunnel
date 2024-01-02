package sentinel

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/USA-RedDragon/sentinel_tunnel/internal/sentinel/resp"
	"golang.org/x/sync/errgroup"
)

const (
	ReadHeaderTimeout = 3 * time.Second
)

type TunnellingClient struct {
	configuration      TunnellingConfiguration
	sentinelConnection *Connection
}

type GetDBAddressByNameFunction func(dbName string) (string, error)

func NewTunnellingClient(config TunnellingConfiguration) (*TunnellingClient, error) {
	tunnellingClient := TunnellingClient{
		configuration: config,
	}

	var err error
	tunnellingClient.sentinelConnection, err = NewConnection(tunnellingClient.configuration)
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

func handleConnection(c net.Conn, client *TunnellingClient, dbName string) {
	response, err := client.sentinelConnection.GetAddressByDbName(dbName)
	if err != nil {
		slog.Error("cannot get db address", "db", dbName, "error", err.Error())
		if response != "" {
			_, _ = c.Write([]byte(resp.SimpleError("ERR Tunnel failed to get db: " + response).String()))
		} else {
			_, _ = c.Write([]byte(resp.SimpleError("ERR Tunnel failed to get db: " + err.Error()).String()))
		}
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

func handleSingleDbConnections(ctx context.Context, client *TunnellingClient, listeningPort string, dbName string) error {
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
		go handleConnection(conn, client, dbName)
	}
}

func (stClient *TunnellingClient) ListenAndServe(ctx context.Context) error {
	group, ctx := errgroup.WithContext(ctx)
	for _, dbConf := range stClient.configuration.Databases {
		dbConf := dbConf
		group.Go(func() error {
			return handleSingleDbConnections(
				ctx,
				stClient,
				dbConf.LocalPort,
				dbConf.Name,
			)
		})
	}

	http.HandleFunc("/health", stClient.Health)
	server := &http.Server{
		Addr:              stClient.configuration.HTTPAddr,
		ReadHeaderTimeout: ReadHeaderTimeout,
	}
	group.Go(func() error {
		if err := server.ListenAndServe(); err != nil {
			return fmt.Errorf("failed to start http server: %w", err)
		}
		return nil
	})

	group.Go(func() error {
		<-ctx.Done()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		//nolint:contextcheck
		if err := server.Shutdown(ctx); err != nil {
			return fmt.Errorf("graceful http shutdown failed: %w", err)
		}
		return nil
	})

	//nolint:wrapcheck
	return group.Wait()
}
