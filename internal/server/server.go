package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"go.uber.org/fx"
	"go.uber.org/zap"

	"nice/internal/config"
	"nice/internal/models"
	"nice/internal/services"
)

type Server struct {
	http   *http.Server
	logger *zap.Logger
}

func New(cfg config.Config, logger *zap.Logger, browse *services.JobBrowse, profile *services.UserProfileService, matches *services.JobMatches) *Server {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/database", databaseHandler(cfg.DatabasePath))
	mux.HandleFunc("GET /api/jobs", jobsHandler(browse, logger))
	mux.HandleFunc("DELETE /api/jobs/{id}", deleteJobHandler(browse, logger))
	mux.HandleFunc("GET /api/jobs/{id}/match", jobMatchHandler(matches, logger))
	mux.HandleFunc("GET /api/providers", providersHandler(browse, logger))
	mux.HandleFunc("GET /api/companies", companiesHandler(browse, logger))
	mux.HandleFunc("GET /api/profile", profileHandler(profile, logger))
	mux.HandleFunc("PUT /api/profile", saveProfileHandler(profile, logger))
	mux.HandleFunc("OPTIONS /api/{path...}", optionsHandler)

	return &Server{
		http: &http.Server{
			Addr:    cfg.HTTPAddress,
			Handler: cors(mux),
		},
		logger: logger,
	}
}

func profileHandler(profile *services.UserProfileService, logger *zap.Logger) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		userProfile, err := profile.Profile(request.Context())
		if err != nil {
			logger.Error("load user profile failed", zap.Error(err))
			writeError(writer, http.StatusInternalServerError, "could not load profile")
			return
		}
		writeJSON(writer, http.StatusOK, map[string]any{"profile": userProfile})
	}
}

func saveProfileHandler(profile *services.UserProfileService, logger *zap.Logger) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		var userProfile models.UserProfile
		if err := json.NewDecoder(request.Body).Decode(&userProfile); err != nil {
			logger.Warn("invalid user profile", zap.Error(err))
			writeError(writer, http.StatusBadRequest, "profile must be valid JSON")
			return
		}
		savedProfile, err := profile.Save(request.Context(), userProfile)
		if err != nil {
			logger.Error("save user profile failed", zap.Error(err))
			writeError(writer, http.StatusInternalServerError, "could not save profile")
			return
		}
		writeJSON(writer, http.StatusOK, map[string]any{"profile": savedProfile})
	}
}

func jobsHandler(browse *services.JobBrowse, logger *zap.Logger) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		jobs, err := browse.Jobs(request.Context(), models.JobSearch{
			Search:   request.URL.Query().Get("search"),
			Provider: request.URL.Query().Get("provider"),
			Fields:   request.URL.Query()["fields"],
		})
		if err != nil {
			if errors.Is(err, services.ErrInvalidSearchField) {
				logger.Warn("invalid job search fields", zap.Error(err))
				writeError(writer, http.StatusBadRequest, services.ErrInvalidSearchField.Error())
				return
			}
			logger.Error("list jobs failed", zap.Error(err))
			writeError(writer, http.StatusInternalServerError, "could not load jobs")
			return
		}
		writeJSON(writer, http.StatusOK, map[string]any{"jobs": jobs})
	}
}

func deleteJobHandler(browse *services.JobBrowse, logger *zap.Logger) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		id, err := strconv.ParseInt(request.PathValue("id"), 10, 64)
		if err != nil || id < 1 {
			logger.Warn("invalid job ID", zap.String("id", request.PathValue("id")))
			writeError(writer, http.StatusBadRequest, "job ID must be a positive integer")
			return
		}
		deleted, err := browse.DeleteJob(request.Context(), id)
		if err != nil {
			logger.Error("delete job failed", zap.Int64("id", id), zap.Error(err))
			writeError(writer, http.StatusInternalServerError, "could not delete job")
			return
		}
		if !deleted {
			writeError(writer, http.StatusNotFound, "job not found")
			return
		}
		writer.WriteHeader(http.StatusNoContent)
	}
}

func jobMatchHandler(matches *services.JobMatches, logger *zap.Logger) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		id, err := strconv.ParseInt(request.PathValue("id"), 10, 64)
		if err != nil || id < 1 {
			logger.Warn("invalid job ID", zap.String("id", request.PathValue("id")))
			writeError(writer, http.StatusBadRequest, "job ID must be a positive integer")
			return
		}
		match, err := matches.Match(request.Context(), id)
		if err != nil {
			logger.Error("load job match failed", zap.Int64("id", id), zap.Error(err))
			writeError(writer, http.StatusInternalServerError, "could not load job match")
			return
		}
		writeJSON(writer, http.StatusOK, map[string]any{"match": match})
	}
}

func providersHandler(browse *services.JobBrowse, logger *zap.Logger) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		providers, err := browse.Providers(request.Context())
		if err != nil {
			logger.Error("list providers failed", zap.Error(err))
			writeError(writer, http.StatusInternalServerError, "could not load providers")
			return
		}
		writeJSON(writer, http.StatusOK, map[string]any{"providers": providers})
	}
}

func companiesHandler(browse *services.JobBrowse, logger *zap.Logger) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		companies, err := browse.Companies(request.Context(), request.URL.Query().Get("search"))
		if err != nil {
			logger.Error("list companies failed", zap.Error(err))
			writeError(writer, http.StatusInternalServerError, "could not load companies")
			return
		}
		writeJSON(writer, http.StatusOK, map[string]any{"companies": companies})
	}
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Access-Control-Allow-Origin", "*")
		writer.Header().Set("Access-Control-Allow-Methods", "GET, PUT, DELETE, OPTIONS")
		writer.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		next.ServeHTTP(writer, request)
	})
}

func optionsHandler(writer http.ResponseWriter, _ *http.Request) {
	writer.WriteHeader(http.StatusNoContent)
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func writeError(writer http.ResponseWriter, status int, message string) {
	writeJSON(writer, status, map[string]string{"error": message})
}

func databaseHandler(path string) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Disposition", `attachment; filename="jobs.db"`)
		writer.Header().Set("Content-Type", "application/vnd.sqlite3")
		http.ServeFile(writer, request, path)
	}
}

func (server *Server) Register(lifecycle fx.Lifecycle) {
	lifecycle.Append(fx.Hook{
		OnStart: func(context.Context) error {
			server.logger.Info("HTTP server starting", zap.String("address", server.http.Addr))
			go func() {
				if err := server.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
					server.logger.Error("HTTP server failed", zap.Error(err))
				}
			}()
			return nil
		},
		OnStop: server.http.Shutdown,
	})
}
