package main

import (
	"context"

	"go.uber.org/fx"

	"nice/internal/clients/adzuna"
	"nice/internal/config"
	"nice/internal/repositories"
	"nice/internal/server"
	"nice/internal/services"
)

func main() {
	fx.New(
		fx.Provide(
			config.Load,
			adzuna.NewClient,
			repositories.NewSQLite,
			services.NewJobSync,
			server.New,
		),
		fx.Invoke(registerLifecycle),
	).Run()
}

func registerLifecycle(
	lifecycle fx.Lifecycle,
	repository repositories.JobRepository,
	server *server.Server,
	sync *services.JobSync,
) {
	lifecycle.Append(fx.Hook{
		OnStop: func(context.Context) error {
			return repository.Close()
		},
	})
	server.Register(lifecycle)
	sync.Register(lifecycle)
}
