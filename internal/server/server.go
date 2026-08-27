package server

import (
	"context"
	"errors"
	"log"
	"net/http"

	"go.uber.org/fx"

	"nice/internal/config"
)

type Server struct {
	http *http.Server
}

func New(cfg config.Config) *Server {
	return &Server{
		http: &http.Server{
			Addr:    cfg.HTTPAddress,
			Handler: http.NotFoundHandler(),
		},
	}
}

func (server *Server) Register(lifecycle fx.Lifecycle) {
	lifecycle.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go func() {
				if err := server.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
					log.Printf("HTTP server failed: %v", err)
				}
			}()
			return nil
		},
		OnStop: server.http.Shutdown,
	})
}
