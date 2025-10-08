package http

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

type Config struct {
	// Address of the WFM API server
	BindAddress string
}

type Server struct {
	srv *http.Server
}

func NewServer(config Config, deploymentHandler DeploymentHandler) *Server {
	mux := http.NewServeMux()

	noContent := func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}

	// Endpoints proposed by the SUP. Those routes are expected
	// to be implemented by compliant WFM API servers.
	mux.HandleFunc("GET /api/v1/devices/{deviceId}/deployments", deploymentHandler.GetDeploymentManifest)
	mux.HandleFunc("GET /api/v1/devices/{deviceId}/deployments/{deploymentId}/{digest}", deploymentHandler.GetDeployment)
	mux.HandleFunc("GET /api/v1/devices/{deviceId}/bundles/{digest}", deploymentHandler.GetBundle)
	// Non-standard endpoints used for demo purposes only. Those routes
	// are NOT expected to be implemented by compliant WFM API servers.
	mux.HandleFunc("POST /api/v1/devices/{deviceId}/deployments", deploymentHandler.CreateDeployment)
	mux.HandleFunc("PUT /api/v1/devices/{deviceId}/deployments/{deploymentId}", deploymentHandler.UpdateDeployment)
	mux.HandleFunc("DELETE /api/v1/devices/{deviceId}/deployments/{deploymentId}", deploymentHandler.DeleteDeployment)
	mux.HandleFunc("GET /healthz", noContent)
	RegisterOpenAPIRoutes(mux)

	srv := &http.Server{
		Addr:              config.BindAddress,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	return &Server{srv}
}

func (s *Server) Run(ctx context.Context) error {
	baseURL := fmt.Sprintf("http://%s", s.srv.Addr)
	logrus.WithFields(logrus.Fields{
		"docs":    baseURL + "/docs",
		"swagger": baseURL + "/swagger",
	}).Info("WFM API ready")

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}
