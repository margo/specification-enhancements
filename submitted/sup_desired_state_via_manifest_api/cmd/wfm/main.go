package main

import (
	"context"
	"os"
	"os/signal"
	"skeleton/pkg/wfm/adapter/persistence/sqlitedb"
	"skeleton/pkg/wfm/adapter/persistence/sqlitedb/repository"
	httptransport "skeleton/pkg/wfm/adapter/transport/http"
	"skeleton/pkg/wfm/core/service"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v3"
)

func run(ctx context.Context, cmd *cli.Command) error {
	bindAddress := cmd.String("bind-address")
	dbPath := cmd.String("db-path")

	// Install signal handler for graceful shutdown
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Initialize database and run migrations
	ds, err := sqlitedb.New(ctx, dbPath)
	if err != nil {
		logrus.WithError(err).Error("Failed to initialize datastore")
		return err
	}
	defer ds.Close()
	if err = ds.Migrate(ctx); err != nil {
		logrus.WithError(err).Error("Failed to migrate database")
		return err
	}

	// Wire the objects
	deploymentRepo := repository.NewDeploymentRepository(ds)
	deploymentSvc := service.NewDeploymentService(deploymentRepo)
	deploymentHandler := httptransport.NewDeploymentHandler(deploymentSvc)

	// Create and run the HTTP server
	s := httptransport.NewServer(httptransport.Config{BindAddress: bindAddress}, *deploymentHandler)

	logrus.WithField("bind_address", bindAddress).Info("Starting HTTP server")
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.Run(ctx)
	}()

	var runErr error
	select {
	case <-ctx.Done():
		runErr = ctx.Err()
	case err := <-errCh:
		runErr = err
		stop()
	}

	if runErr != nil && runErr != context.Canceled {
		logrus.WithError(runErr).Error("Server quit unexpectedly")
	}

	logrus.Info("Shutting down server")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.Shutdown(shutdownCtx); err != nil {
		logrus.WithError(err).Warn("Server shutdown timeout or error")
	}

	return nil
}

func main() {
	cmd := &cli.Command{
		Name:  "wfm",
		Usage: "Workload Fleet Management API Server",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "bind-address",
				Value: ":8080",
				Usage: "The IP address and port on which to serve the API (host:port)",
			},
			&cli.StringFlag{
				Name:  "db-path",
				Value: "./wfm.db",
				Usage: "Path to the SQLite database",
			},
		},
		Action: run,
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		logrus.Fatal(err)
	}
}
