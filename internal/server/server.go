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
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/database", databaseHandler(cfg.Database.Path))

	return &Server{
		http: &http.Server{
			Addr:    cfg.HTTPAddress,
			Handler: mux,
		},
	}
}

func databaseHandler(path string) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Access-Control-Allow-Origin", "*")
		writer.Header().Set("Content-Disposition", `attachment; filename="jobs.db"`)
		writer.Header().Set("Content-Type", "application/vnd.sqlite3")
		http.ServeFile(writer, request, path)
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
