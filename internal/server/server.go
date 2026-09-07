package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"go.uber.org/fx"
	"go.uber.org/zap"

	"nice/internal/config"
	"nice/internal/models"
	"nice/internal/services"
)

type Server struct {
	http              *http.Server
	metrics           *http.Server
	metricsSocketPath string
	listener          net.Listener
	logger            *zap.Logger
}

type syncTrigger interface {
	Trigger(string) bool
}

func New(cfg config.Config, logger *zap.Logger, browse *services.JobBrowse, events *services.EventLog, settings *services.DiscoverySettingsService, syncer *services.JobSync, previews *services.ProviderPreviewService, profile *services.UserProfileService, resume *services.ResumeService, resumePDF *services.ResumePDFService, matches *services.JobMatches, requests *services.JobMatchRequests, chat *services.JobMatchChat, metrics *services.LinkedInMetrics) *Server {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/database", databaseHandler(cfg.DatabasePath))
	mux.HandleFunc("GET /api/jobs", jobsHandler(browse, logger))
	mux.HandleFunc("GET /api/jobs/{id}", jobHandler(browse, logger))
	mux.HandleFunc("DELETE /api/jobs/{id}", deleteJobHandler(browse, logger))
	mux.HandleFunc("GET /api/jobs/{id}/match", jobMatchHandler(matches, logger))
	mux.HandleFunc("GET /api/jobs/{id}/match/resume.pdf", jobApplicationResumePDFHandler(chat, resumePDF, logger))
	mux.HandleFunc("POST /api/jobs/{id}/match", queueJobMatchHandler(requests, logger, false))
	mux.HandleFunc("POST /api/jobs/{id}/match/redo", queueJobMatchHandler(requests, logger, true))
	mux.HandleFunc("GET /api/jobs/{id}/match/chat", jobMatchChatItemsHandler(chat, logger))
	mux.HandleFunc("GET /api/jobs/{id}/match/chat/events", jobMatchChatEventsHandler(chat, logger))
	mux.HandleFunc("POST /api/jobs/{id}/match/chat", jobMatchChatSendHandler(chat, logger))
	mux.HandleFunc("POST /api/jobs/{id}/match/chat/{requestID}/stop", jobMatchChatStopHandler(chat, logger))
	mux.HandleFunc("DELETE /api/jobs/{id}/match/chat/{sequence}", jobMatchChatRevertHandler(chat, logger))
	mux.HandleFunc("GET /api/matches", jobMatchesHandler(matches, logger))
	mux.HandleFunc("GET /api/match-queue", matchQueueHandler(requests, logger))
	mux.HandleFunc("POST /api/match-queue", queueUnmatchedJobMatchesHandler(requests, logger))
	mux.HandleFunc("PUT /api/match-queue", reorderMatchQueueHandler(requests, logger))
	mux.HandleFunc("DELETE /api/match-queue/{id}", removeMatchQueueHandler(requests, logger))
	mux.HandleFunc("GET /api/providers", providersHandler(browse, logger))
	mux.HandleFunc("GET /api/companies", companiesHandler(browse, logger))
	mux.HandleFunc("GET /api/events", eventsHandler(events, logger))
	mux.HandleFunc("GET /api/discovery-settings", discoverySettingsHandler(settings, logger))
	mux.HandleFunc("PUT /api/discovery-settings", saveDiscoverySettingsHandler(settings, logger))
	mux.HandleFunc("POST /api/sync/{provider}", discoverySyncHandler(syncer, logger))
	mux.HandleFunc("POST /api/discovery-preview/{provider}", discoveryPreviewHandler(previews, logger))
	mux.HandleFunc("GET /api/profile", profileHandler(profile, logger))
	mux.HandleFunc("PUT /api/profile", saveProfileHandler(profile, logger))
	mux.HandleFunc("GET /api/resume", resumeHandler(resume, logger))
	mux.HandleFunc("PUT /api/resume", saveResumeHandler(resume, logger))
	mux.HandleFunc("GET /api/resume.pdf", resumePDFHandler(resumePDF, logger))
	mux.HandleFunc("GET /api/resume/photo", resumePhotoHandler(resume, logger))
	mux.HandleFunc("POST /api/resume/photo", saveResumePhotoHandler(resume, logger))
	mux.HandleFunc("OPTIONS /api/{path...}", optionsHandler)

	return &Server{
		http: &http.Server{
			Addr:    cfg.HTTPAddress,
			Handler: cors(mux),
		},
		logger: logger,
		metrics: &http.Server{
			Handler: metrics.Handler(),
		},
		metricsSocketPath: cfg.MetricsSocketPath,
	}
}

type resumePDFGenerator interface {
	Generate(context.Context) ([]byte, error)
}

type applicationResumePDFGenerator interface {
	GenerateResume(context.Context, models.Resume) ([]byte, error)
}

type applicationResumeProvider interface {
	LatestResume(context.Context, int64) (*models.Resume, error)
}

func resumePDFHandler(pdf resumePDFGenerator, logger *zap.Logger) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		content, err := pdf.Generate(request.Context())
		if errors.Is(err, services.ErrResumeNotFound) {
			logger.Warn("generate base resume PDF without a resume", zap.Error(err))
			writeError(writer, http.StatusNotFound, "base resume not found")
			return
		}
		if err != nil {
			logger.Error("generate base resume PDF failed", zap.Error(err))
			writeError(writer, http.StatusInternalServerError, "could not generate resume PDF")
			return
		}
		writer.Header().Set("Cache-Control", "no-store")
		writer.Header().Set("Content-Disposition", `attachment; filename="resume.pdf"`)
		writer.Header().Set("Content-Type", "application/pdf")
		writer.WriteHeader(http.StatusOK)
		if _, err := writer.Write(content); err != nil {
			logger.Error("write base resume PDF failed", zap.Error(err))
		}
	}
}

func jobApplicationResumePDFHandler(chat applicationResumeProvider, pdf applicationResumePDFGenerator, logger *zap.Logger) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		jobID, err := strconv.ParseInt(request.PathValue("id"), 10, 64)
		if err != nil || jobID < 1 {
			logger.Warn("invalid job ID for application resume PDF", zap.String("id", request.PathValue("id")))
			writeError(writer, http.StatusBadRequest, "job ID must be a positive integer")
			return
		}
		resume, err := chat.LatestResume(request.Context(), jobID)
		if errors.Is(err, services.ErrApplicationResumeNotFound) {
			logger.Warn("generate application resume PDF without a resume", zap.Int64("job_id", jobID), zap.Error(err))
			writeError(writer, http.StatusNotFound, "application resume not found")
			return
		}
		if err != nil {
			logger.Error("load latest application resume failed", zap.Int64("job_id", jobID), zap.Error(err))
			writeError(writer, http.StatusInternalServerError, "could not load application resume")
			return
		}
		content, err := pdf.GenerateResume(request.Context(), *resume)
		if err != nil {
			logger.Error("generate application resume PDF failed", zap.Int64("job_id", jobID), zap.Error(err))
			writeError(writer, http.StatusInternalServerError, "could not generate application resume PDF")
			return
		}
		writer.Header().Set("Cache-Control", "no-store")
		writer.Header().Set("Content-Disposition", `attachment; filename="application-resume.pdf"`)
		writer.Header().Set("Content-Type", "application/pdf")
		writer.WriteHeader(http.StatusOK)
		if _, err := writer.Write(content); err != nil {
			logger.Error("write application resume PDF failed", zap.Int64("job_id", jobID), zap.Error(err))
		}
	}
}

func resumePhotoHandler(resume *services.ResumeService, logger *zap.Logger) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		photo, err := resume.Photo(request.Context())
		if errors.Is(err, sql.ErrNoRows) {
			logger.Warn("load base resume photo without a photo", zap.Error(err))
			writeError(writer, http.StatusNotFound, "resume photo not found")
			return
		}
		if err != nil {
			logger.Error("load base resume photo failed", zap.Error(err))
			writeError(writer, http.StatusInternalServerError, "could not load resume photo")
			return
		}
		writer.Header().Set("Cache-Control", "no-store")
		writer.Header().Set("Content-Type", photo.ContentType)
		if _, err := writer.Write(photo.Data); err != nil {
			logger.Error("write base resume photo failed", zap.Error(err))
		}
	}
}

func saveResumePhotoHandler(resume *services.ResumeService, logger *zap.Logger) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		request.Body = http.MaxBytesReader(writer, request.Body, services.MaxResumePhotoBytes+1024)
		file, _, err := request.FormFile("photo")
		if err != nil {
			logger.Warn("read base resume photo failed", zap.Error(err))
			writeError(writer, http.StatusBadRequest, "photo is required and must be smaller than 5 MB")
			return
		}
		defer file.Close()
		data, err := io.ReadAll(file)
		if err != nil {
			logger.Error("read base resume photo data failed", zap.Error(err))
			writeError(writer, http.StatusInternalServerError, "could not read resume photo")
			return
		}
		saved, err := resume.SavePhoto(request.Context(), data)
		if errors.Is(err, services.ErrInvalidResumePhoto) {
			logger.Warn("invalid base resume photo", zap.Error(err))
			writeError(writer, http.StatusBadRequest, err.Error())
			return
		}
		if errors.Is(err, sql.ErrNoRows) {
			logger.Warn("save base resume photo without a resume", zap.Error(err))
			writeError(writer, http.StatusNotFound, "base resume not found")
			return
		}
		if err != nil {
			logger.Error("save base resume photo failed", zap.Error(err))
			writeError(writer, http.StatusInternalServerError, "could not save resume photo")
			return
		}
		writeJSON(writer, http.StatusOK, map[string]any{"resume": saved})
	}
}

func eventsHandler(events *services.EventLog, logger *zap.Logger) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		limit, err := paginationQueryInt(request, "limit", 50)
		if err != nil {
			logger.Warn("invalid events limit", zap.Error(err))
			writeError(writer, http.StatusBadRequest, "limit must be a positive integer no greater than 100")
			return
		}
		offset, err := paginationQueryInt(request, "offset", 0)
		if err != nil {
			logger.Warn("invalid events offset", zap.Error(err))
			writeError(writer, http.StatusBadRequest, "offset must be a non-negative integer")
			return
		}
		page, err := events.Events(request.Context(), models.EventSearch{
			Provider: strings.TrimSpace(request.URL.Query().Get("provider")),
			RunID:    strings.TrimSpace(request.URL.Query().Get("runId")),
			Type:     strings.TrimSpace(request.URL.Query().Get("type")),
			Level:    strings.TrimSpace(request.URL.Query().Get("level")),
			Limit:    limit,
			Offset:   offset,
		})
		if errors.Is(err, services.ErrInvalidEventSearch) {
			logger.Warn("invalid event search", zap.Error(err))
			writeError(writer, http.StatusBadRequest, "invalid event search")
			return
		}
		if err != nil {
			logger.Error("list events failed", zap.Error(err))
			writeError(writer, http.StatusInternalServerError, "could not load events")
			return
		}
		writeJSON(writer, http.StatusOK, page)
	}
}

func paginationQueryInt(request *http.Request, name string, fallback int) (int, error) {
	value := request.URL.Query().Get(name)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, err
	}
	if name == "limit" && (parsed < 1 || parsed > 100) {
		return 0, errors.New("limit out of range")
	}
	if name == "offset" && parsed < 0 {
		return 0, errors.New("negative offset")
	}
	return parsed, nil
}

type discoveryPreviewer interface {
	Preview(context.Context, string, models.DiscoverySettings) ([]models.Job, error)
	StreamPreview(context.Context, string, models.DiscoverySettings, func(models.Job) error) error
}

func discoveryPreviewHandler(previews discoveryPreviewer, logger *zap.Logger) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		var settings models.DiscoverySettings
		if err := json.NewDecoder(request.Body).Decode(&settings); err != nil {
			logger.Warn("invalid discovery preview settings", zap.Error(err))
			writeError(writer, http.StatusBadRequest, "settings must be valid JSON")
			return
		}

		provider := request.PathValue("provider")
		if request.Header.Get("Accept") == "text/event-stream" {
			discoveryPreviewStreamHandler(writer, request, previews, logger, provider, settings)
			return
		}

		jobs, err := previews.Preview(request.Context(), provider, settings)
		if err != nil {
			writeDiscoveryPreviewError(writer, logger, provider, err)
			return
		}
		if jobs == nil {
			jobs = []models.Job{}
		}

		logger.Info("discovery preview fetched", zap.String("provider", provider), zap.Int("job_count", len(jobs)))
		writeJSON(writer, http.StatusOK, map[string]any{"jobs": jobs})
	}
}

func discoveryPreviewStreamHandler(writer http.ResponseWriter, request *http.Request, previews discoveryPreviewer, logger *zap.Logger, provider string, settings models.DiscoverySettings) {
	flusher, ok := writer.(http.Flusher)
	if !ok {
		logger.Error("discovery preview SSE is not supported", zap.String("provider", provider))
		writeError(writer, http.StatusInternalServerError, "discovery preview events are unavailable")
		return
	}
	writer.Header().Set("Cache-Control", "no-cache")
	writer.Header().Set("Connection", "keep-alive")
	writer.Header().Set("Content-Type", "text/event-stream")
	flusher.Flush()

	jobCount := 0
	err := previews.StreamPreview(request.Context(), provider, settings, func(job models.Job) error {
		if err := writeSSE(writer, "job", job); err != nil {
			return err
		}
		jobCount++
		flusher.Flush()
		return nil
	})
	if err != nil {
		if errors.Is(err, services.ErrUnknownDiscoveryProvider) || errors.Is(err, services.ErrInvalidDiscoverySettings) {
			logger.Warn("discovery preview rejected", zap.String("provider", provider), zap.Error(err))
			if writeErr := writeSSE(writer, "error", map[string]string{"error": err.Error()}); writeErr != nil {
				logger.Error("write discovery preview SSE error failed", zap.String("provider", provider), zap.Error(writeErr))
			}
		} else {
			logger.Error("discovery preview failed", zap.String("provider", provider), zap.Error(err))
			if writeErr := writeSSE(writer, "error", map[string]string{"error": "could not fetch provider preview"}); writeErr != nil {
				logger.Error("write discovery preview SSE error failed", zap.String("provider", provider), zap.Error(writeErr))
			}
		}
		flusher.Flush()
		return
	}

	logger.Info("discovery preview fetched", zap.String("provider", provider), zap.Int("job_count", jobCount))
	if err := writeSSE(writer, "complete", map[string]int{"jobCount": jobCount}); err != nil {
		logger.Error("write discovery preview SSE completion failed", zap.String("provider", provider), zap.Error(err))
		return
	}
	flusher.Flush()
}

func writeDiscoveryPreviewError(writer http.ResponseWriter, logger *zap.Logger, provider string, err error) {
	if errors.Is(err, services.ErrUnknownDiscoveryProvider) || errors.Is(err, services.ErrInvalidDiscoverySettings) {
		logger.Warn("discovery preview rejected", zap.String("provider", provider), zap.Error(err))
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	logger.Error("discovery preview failed", zap.String("provider", provider), zap.Error(err))
	writeError(writer, http.StatusBadGateway, "could not fetch provider preview")
}

func writeSSE(writer io.Writer, event string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(writer, "event: %s\ndata: %s\n\n", event, data)
	return err
}

func discoverySettingsHandler(settings *services.DiscoverySettingsService, logger *zap.Logger) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		value, err := settings.Settings(request.Context())
		if err != nil {
			logger.Error("load discovery settings failed", zap.Error(err))
			writeError(writer, http.StatusInternalServerError, "could not load discovery settings")
			return
		}
		writeJSON(writer, http.StatusOK, map[string]any{"settings": value})
	}
}

func saveDiscoverySettingsHandler(settings *services.DiscoverySettingsService, logger *zap.Logger) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		var value models.DiscoverySettings
		if err := json.NewDecoder(request.Body).Decode(&value); err != nil {
			logger.Warn("invalid discovery settings", zap.Error(err))
			writeError(writer, http.StatusBadRequest, "settings must be valid JSON")
			return
		}
		saved, err := settings.Save(request.Context(), value)
		if err != nil {
			if errors.Is(err, services.ErrInvalidDiscoverySettings) {
				logger.Warn("invalid discovery settings", zap.Error(err))
				writeError(writer, http.StatusBadRequest, err.Error())
				return
			}
			logger.Error("save discovery settings failed", zap.Error(err))
			writeError(writer, http.StatusInternalServerError, "could not save discovery settings")
			return
		}
		writeJSON(writer, http.StatusOK, map[string]any{"settings": saved})
	}
}

func discoverySyncHandler(syncer syncTrigger, logger *zap.Logger) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		provider := request.PathValue("provider")
		if !isDiscoveryProvider(provider) {
			logger.Warn("invalid discovery sync provider", zap.String("provider", provider))
			writeError(writer, http.StatusBadRequest, "unknown discovery provider")
			return
		}
		started := syncer.Trigger(provider)
		logger.Info("discovery sync requested", zap.String("provider", provider), zap.Bool("started", started))
		writeJSON(writer, http.StatusAccepted, map[string]bool{"started": started})
	}
}

func isDiscoveryProvider(provider string) bool {
	switch provider {
	case "adzuna", "jobicy", "linkedin", "remotive":
		return true
	default:
		return false
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

func resumeHandler(resume *services.ResumeService, logger *zap.Logger) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		value, err := resume.Resume(request.Context())
		if err != nil {
			logger.Error("load base resume failed", zap.Error(err))
			writeError(writer, http.StatusInternalServerError, "could not load resume")
			return
		}
		writeJSON(writer, http.StatusOK, map[string]any{"resume": value})
	}
}

func saveResumeHandler(resume *services.ResumeService, logger *zap.Logger) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		var value models.Resume
		if err := json.NewDecoder(request.Body).Decode(&value); err != nil {
			logger.Warn("invalid base resume", zap.Error(err))
			writeError(writer, http.StatusBadRequest, "resume must be valid JSON")
			return
		}
		saved, err := resume.Save(request.Context(), value)
		if err != nil {
			logger.Error("save base resume failed", zap.Error(err))
			writeError(writer, http.StatusInternalServerError, "could not save resume")
			return
		}
		writeJSON(writer, http.StatusOK, map[string]any{"resume": saved})
	}
}

func jobsHandler(browse *services.JobBrowse, logger *zap.Logger) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		limit, err := paginationQueryInt(request, "limit", 50)
		if err != nil {
			logger.Warn("invalid jobs limit", zap.Error(err))
			writeError(writer, http.StatusBadRequest, "limit must be a positive integer no greater than 100")
			return
		}
		offset, err := paginationQueryInt(request, "offset", 0)
		if err != nil {
			logger.Warn("invalid jobs offset", zap.Error(err))
			writeError(writer, http.StatusBadRequest, "offset must be a non-negative integer")
			return
		}
		page, err := browse.Jobs(request.Context(), models.JobSearch{
			Search:   request.URL.Query().Get("search"),
			Provider: request.URL.Query().Get("provider"),
			Match:    request.URL.Query().Get("match"),
			Fields:   request.URL.Query()["fields"],
			Limit:    limit,
			Offset:   offset,
		})
		if err != nil {
			if errors.Is(err, services.ErrInvalidSearchField) || errors.Is(err, services.ErrInvalidMatchFilter) {
				logger.Warn("invalid job search", zap.Error(err))
				writeError(writer, http.StatusBadRequest, err.Error())
				return
			}
			logger.Error("list jobs failed", zap.Error(err))
			writeError(writer, http.StatusInternalServerError, "could not load jobs")
			return
		}
		writeJSON(writer, http.StatusOK, page)
	}
}

func jobHandler(browse *services.JobBrowse, logger *zap.Logger) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		id, err := strconv.ParseInt(request.PathValue("id"), 10, 64)
		if err != nil || id < 1 {
			logger.Warn("invalid job ID", zap.String("id", request.PathValue("id")))
			writeError(writer, http.StatusBadRequest, "job ID must be a positive integer")
			return
		}
		job, err := browse.Job(request.Context(), id)
		if errors.Is(err, sql.ErrNoRows) {
			writeError(writer, http.StatusNotFound, "job not found")
			return
		}
		if err != nil {
			logger.Error("load job failed", zap.Int64("id", id), zap.Error(err))
			writeError(writer, http.StatusInternalServerError, "could not load job")
			return
		}
		writeJSON(writer, http.StatusOK, map[string]any{"job": job})
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
		analysis, err := matches.Analysis(request.Context(), id)
		if err != nil {
			logger.Error("load job analysis failed", zap.Int64("id", id), zap.Error(err))
			writeError(writer, http.StatusInternalServerError, "could not load job analysis")
			return
		}
		writeJSON(writer, http.StatusOK, map[string]any{"match": match, "analysis": analysis})
	}
}

func jobMatchesHandler(matches *services.JobMatches, logger *zap.Logger) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		listed, err := matches.List(request.Context())
		if err != nil {
			logger.Error("list job matches failed", zap.Error(err))
			writeError(writer, http.StatusInternalServerError, "could not load job matches")
			return
		}
		writeJSON(writer, http.StatusOK, map[string]any{"matches": listed})
	}
}

func jobMatchChatItemsHandler(chat *services.JobMatchChat, logger *zap.Logger) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		id, err := strconv.ParseInt(request.PathValue("id"), 10, 64)
		if err != nil || id < 1 {
			logger.Warn("invalid job ID for match chat", zap.String("id", request.PathValue("id")))
			writeError(writer, http.StatusBadRequest, "job ID must be a positive integer")
			return
		}
		after, err := chatAfterSequence(request)
		if err != nil {
			logger.Warn("invalid match chat item sequence", zap.Int64("id", id), zap.Error(err))
			writeError(writer, http.StatusBadRequest, "after must be a non-negative integer")
			return
		}
		items, err := chat.Items(request.Context(), id, after)
		if err != nil {
			logger.Error("load job match chat items failed", zap.Int64("id", id), zap.Error(err))
			writeError(writer, http.StatusInternalServerError, "could not load match chat")
			return
		}
		writeJSON(writer, http.StatusOK, map[string]any{"items": items})
	}
}

func chatAfterSequence(request *http.Request) (int64, error) {
	value := request.URL.Query().Get("after")
	if value == "" {
		return 0, nil
	}
	after, err := strconv.ParseInt(value, 10, 64)
	if err != nil || after < 0 {
		return 0, errors.New("invalid after sequence")
	}
	return after, nil
}

func jobMatchChatEventsHandler(chat *services.JobMatchChat, logger *zap.Logger) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		id, err := strconv.ParseInt(request.PathValue("id"), 10, 64)
		if err != nil || id < 1 {
			logger.Warn("invalid job ID for match chat events", zap.String("id", request.PathValue("id")))
			writeError(writer, http.StatusBadRequest, "job ID must be a positive integer")
			return
		}
		after, err := chatAfterSequence(request)
		if err != nil {
			logger.Warn("invalid match chat event sequence", zap.Int64("id", id), zap.Error(err))
			writeError(writer, http.StatusBadRequest, "after must be a non-negative integer")
			return
		}
		flusher, ok := writer.(http.Flusher)
		if !ok {
			logger.Error("match chat SSE is not supported", zap.Int64("id", id))
			writeError(writer, http.StatusInternalServerError, "match chat events are unavailable")
			return
		}
		writer.Header().Set("Cache-Control", "no-cache")
		writer.Header().Set("Connection", "keep-alive")
		writer.Header().Set("Content-Type", "text/event-stream")
		updates, unsubscribe := chat.Subscribe(id)
		defer unsubscribe()
		writeChatItems := func() bool {
			items, err := chat.Items(request.Context(), id, after)
			if err != nil {
				logger.Error("load match chat SSE items failed", zap.Int64("id", id), zap.Error(err))
				return false
			}
			for _, item := range items {
				data, err := json.Marshal(item)
				if err != nil {
					logger.Error("encode match chat SSE item failed", zap.Int64("id", id), zap.Error(err))
					return false
				}
				if _, err := writer.Write([]byte("event: item\ndata: " + string(data) + "\n\n")); err != nil {
					logger.Error("write match chat SSE item failed", zap.Int64("id", id), zap.Error(err))
					return false
				}
				after = item.Sequence
			}
			flusher.Flush()
			return true
		}
		if !writeChatItems() {
			return
		}
		for {
			select {
			case <-request.Context().Done():
				return
			case update := <-updates:
				if update.Reset {
					if _, err := writer.Write([]byte("event: reset\ndata: {}\n\n")); err != nil {
						logger.Error("write match chat SSE reset failed", zap.Int64("id", id), zap.Error(err))
						return
					}
					flusher.Flush()
					continue
				}
				if !writeChatItems() {
					return
				}
			}
		}
	}
}

func jobMatchChatSendHandler(chat *services.JobMatchChat, logger *zap.Logger) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		id, err := strconv.ParseInt(request.PathValue("id"), 10, 64)
		if err != nil || id < 1 {
			logger.Warn("invalid job ID for match chat", zap.String("id", request.PathValue("id")))
			writeError(writer, http.StatusBadRequest, "job ID must be a positive integer")
			return
		}
		var body struct {
			Content   string `json:"content"`
			RequestID string `json:"requestId"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			logger.Warn("invalid job match chat message", zap.Int64("id", id), zap.Error(err))
			writeError(writer, http.StatusBadRequest, "message must be valid JSON")
			return
		}
		item, err := chat.Send(request.Context(), id, body.Content, body.RequestID)
		if errors.Is(err, services.ErrEmptyJobMatchChatMessage) || errors.Is(err, services.ErrMissingJobMatchChatRequestID) {
			logger.Warn("empty job match chat message", zap.Int64("id", id), zap.Error(err))
			writeError(writer, http.StatusBadRequest, err.Error())
			return
		}
		if errors.Is(err, services.ErrJobMatchChatUnavailable) {
			logger.Warn("job match chat unavailable", zap.Int64("id", id), zap.Error(err))
			writeError(writer, http.StatusConflict, err.Error())
			return
		}
		if errors.Is(err, services.ErrJobMatchChatTurnActive) || errors.Is(err, services.ErrJobMatchChatUnansweredMessage) {
			logger.Warn("job match chat turn rejected", zap.Int64("id", id), zap.Error(err))
			writeError(writer, http.StatusConflict, err.Error())
			return
		}
		if err != nil {
			logger.Error("reply to job match chat failed", zap.Int64("id", id), zap.Error(err))
			writeError(writer, http.StatusInternalServerError, "could not reply to match chat")
			return
		}
		writeJSON(writer, http.StatusAccepted, map[string]any{"item": item})
	}
}

func jobMatchChatStopHandler(chat *services.JobMatchChat, logger *zap.Logger) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		jobID, err := strconv.ParseInt(request.PathValue("id"), 10, 64)
		if err != nil || jobID < 1 {
			logger.Warn("invalid job ID for match chat stop", zap.String("id", request.PathValue("id")))
			writeError(writer, http.StatusBadRequest, "job ID must be a positive integer")
			return
		}
		requestID := strings.TrimSpace(request.PathValue("requestID"))
		if requestID == "" {
			logger.Warn("missing match chat request ID for stop", zap.Int64("job_id", jobID))
			writeError(writer, http.StatusBadRequest, "request ID is required")
			return
		}
		if !chat.Stop(jobID, requestID) {
			logger.Warn("active match chat turn not found for stop", zap.Int64("job_id", jobID), zap.String("request_id", requestID))
			writeError(writer, http.StatusNotFound, "active chat turn not found")
			return
		}
		writer.WriteHeader(http.StatusNoContent)
	}
}

func jobMatchChatRevertHandler(chat *services.JobMatchChat, logger *zap.Logger) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		jobID, err := strconv.ParseInt(request.PathValue("id"), 10, 64)
		if err != nil || jobID < 1 {
			logger.Warn("invalid job ID for match chat revert", zap.String("id", request.PathValue("id")))
			writeError(writer, http.StatusBadRequest, "job ID must be a positive integer")
			return
		}
		sequence, err := strconv.ParseInt(request.PathValue("sequence"), 10, 64)
		if err != nil || sequence < 1 {
			logger.Warn("invalid item sequence for match chat revert", zap.Int64("job_id", jobID), zap.String("sequence", request.PathValue("sequence")))
			writeError(writer, http.StatusBadRequest, "item sequence must be a positive integer")
			return
		}
		if err := chat.Revert(request.Context(), jobID, sequence); err != nil {
			if errors.Is(err, services.ErrJobMatchChatUserMessageNotFound) {
				logger.Warn("user chat item not found for revert", zap.Int64("job_id", jobID), zap.Int64("sequence", sequence), zap.Error(err))
				writeError(writer, http.StatusNotFound, err.Error())
				return
			}
			logger.Error("revert job match chat failed", zap.Int64("job_id", jobID), zap.Int64("sequence", sequence), zap.Error(err))
			writeError(writer, http.StatusInternalServerError, "could not revert match chat")
			return
		}
		writer.WriteHeader(http.StatusNoContent)
	}
}

func queueJobMatchHandler(requests *services.JobMatchRequests, logger *zap.Logger, redo bool) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		id, err := strconv.ParseInt(request.PathValue("id"), 10, 64)
		if err != nil || id < 1 {
			logger.Warn("invalid job ID", zap.String("id", request.PathValue("id")))
			writeError(writer, http.StatusBadRequest, "job ID must be a positive integer")
			return
		}
		if err := requests.Queue(request.Context(), id, redo); err != nil {
			if errors.Is(err, services.ErrMatchJobNotFound) {
				writeError(writer, http.StatusNotFound, "job not found")
				return
			}
			logger.Error("queue job match failed", zap.Int64("id", id), zap.Bool("redo", redo), zap.Error(err))
			writeError(writer, http.StatusInternalServerError, "could not queue job match")
			return
		}
		writeJSON(writer, http.StatusAccepted, map[string]bool{"queued": true})
	}
}

func matchQueueHandler(requests *services.JobMatchRequests, logger *zap.Logger) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		queue, err := requests.List(request.Context())
		if err != nil {
			logger.Error("load job match queue failed", zap.Error(err))
			writeError(writer, http.StatusInternalServerError, "could not load job match queue")
			return
		}
		writeJSON(writer, http.StatusOK, map[string]any{"jobs": queue})
	}
}

func queueUnmatchedJobMatchesHandler(requests *services.JobMatchRequests, logger *zap.Logger) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		var body struct {
			JobIDs []int64 `json:"jobIds"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			logger.Warn("invalid unmatched job match queue", zap.Error(err))
			writeError(writer, http.StatusBadRequest, "jobIds must be valid JSON")
			return
		}
		for _, id := range body.JobIDs {
			if id < 1 {
				logger.Warn("invalid unmatched job match queue ID", zap.Int64("id", id))
				writeError(writer, http.StatusBadRequest, "job IDs must be positive integers")
				return
			}
		}
		queued, err := requests.QueueUnmatched(request.Context(), body.JobIDs)
		if err != nil {
			logger.Error("queue unmatched job matches failed", zap.Int64s("job_ids", body.JobIDs), zap.Error(err))
			writeError(writer, http.StatusInternalServerError, "could not queue unmatched job matches")
			return
		}
		writeJSON(writer, http.StatusAccepted, map[string]int{"queued": queued})
	}
}

func reorderMatchQueueHandler(requests *services.JobMatchRequests, logger *zap.Logger) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		var body struct {
			JobIDs []int64 `json:"jobIds"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			logger.Warn("invalid job match queue", zap.Error(err))
			writeError(writer, http.StatusBadRequest, "jobIds must be valid JSON")
			return
		}
		for _, id := range body.JobIDs {
			if id < 1 {
				writeError(writer, http.StatusBadRequest, "job IDs must be positive integers")
				return
			}
		}
		if err := requests.Reorder(request.Context(), body.JobIDs); err != nil {
			if errors.Is(err, services.ErrMatchJobNotFound) {
				writeError(writer, http.StatusNotFound, "a queued job was not found")
				return
			}
			logger.Error("reorder job match queue failed", zap.Error(err))
			writeError(writer, http.StatusInternalServerError, "could not reorder job match queue")
			return
		}
		writeJSON(writer, http.StatusOK, map[string]bool{"queued": true})
	}
}

func removeMatchQueueHandler(requests *services.JobMatchRequests, logger *zap.Logger) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		id, err := strconv.ParseInt(request.PathValue("id"), 10, 64)
		if err != nil || id < 1 {
			logger.Warn("invalid match queue job ID", zap.String("id", request.PathValue("id")))
			writeError(writer, http.StatusBadRequest, "job ID must be a positive integer")
			return
		}
		if err := requests.Remove(request.Context(), id); err != nil {
			logger.Error("remove job from match queue failed", zap.Int64("id", id), zap.Error(err))
			writeError(writer, http.StatusInternalServerError, "could not remove job from match queue")
			return
		}
		writer.WriteHeader(http.StatusNoContent)
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
		writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
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
			listener, err := listenMetrics(server.metricsSocketPath)
			if err != nil {
				server.logger.Error("metrics server start failed", zap.String("path", server.metricsSocketPath), zap.Error(err))
				return err
			}
			server.listener = listener
			server.logger.Info("metrics server starting", zap.String("path", server.metricsSocketPath))
			go func() {
				if err := server.metrics.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
					server.logger.Error("metrics server failed", zap.Error(err))
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			if err := server.http.Shutdown(ctx); err != nil {
				server.logger.Error("HTTP server shutdown failed", zap.Error(err))
				return err
			}
			if err := server.metrics.Shutdown(ctx); err != nil {
				server.logger.Error("metrics server shutdown failed", zap.Error(err))
				return err
			}
			if server.listener != nil {
				if err := os.Remove(server.listener.Addr().String()); err != nil && !errors.Is(err, os.ErrNotExist) {
					server.logger.Error("remove metrics socket failed", zap.Error(err))
					return err
				}
			}
			return nil
		},
	})
}

func listenMetrics(path string) (net.Listener, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, fmt.Errorf("create metrics socket directory: %w", err)
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("remove stale metrics socket: %w", err)
	}
	listener, err := net.Listen("unix", path)
	if err != nil {
		return nil, fmt.Errorf("listen on metrics socket: %w", err)
	}
	if err := os.Chmod(path, 0o660); err != nil {
		listener.Close()
		return nil, fmt.Errorf("set metrics socket permissions: %w", err)
	}
	return listener, nil
}
