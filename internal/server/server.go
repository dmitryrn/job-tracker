package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

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

func New(cfg config.Config, logger *zap.Logger, browse *services.JobBrowse, events *services.EventLog, settings *services.DiscoverySettingsService, previews *services.ProviderPreviewService, profile *services.UserProfileService, resume *services.ResumeService, resumePDF *services.ResumePDFService, matches *services.JobMatches, requests *services.JobMatchRequests, chat *services.JobMatchChat) *Server {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/database", databaseHandler(cfg.DatabasePath))
	mux.HandleFunc("GET /api/jobs", jobsHandler(browse, logger))
	mux.HandleFunc("GET /api/jobs/{id}", jobHandler(browse, logger))
	mux.HandleFunc("DELETE /api/jobs/{id}", deleteJobHandler(browse, logger))
	mux.HandleFunc("GET /api/jobs/{id}/match", jobMatchHandler(matches, logger))
	mux.HandleFunc("POST /api/jobs/{id}/match", queueJobMatchHandler(requests, logger, false))
	mux.HandleFunc("POST /api/jobs/{id}/match/redo", queueJobMatchHandler(requests, logger, true))
	mux.HandleFunc("GET /api/jobs/{id}/match/chat", jobMatchChatMessagesHandler(chat, logger))
	mux.HandleFunc("GET /api/jobs/{id}/match/application-resume", jobMatchApplicationResumeHandler(chat, logger))
	mux.HandleFunc("POST /api/jobs/{id}/match/chat", jobMatchChatReplyHandler(chat, logger))
	mux.HandleFunc("DELETE /api/jobs/{id}/match/chat/{messageID}", jobMatchChatRevertHandler(chat, logger))
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
	}
}

type resumePDFGenerator interface {
	Generate(context.Context) ([]byte, error)
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
		limit, err := eventQueryInt(request, "limit", 50)
		if err != nil {
			logger.Warn("invalid events limit", zap.Error(err))
			writeError(writer, http.StatusBadRequest, "limit must be a positive integer no greater than 100")
			return
		}
		offset, err := eventQueryInt(request, "offset", 0)
		if err != nil {
			logger.Warn("invalid events offset", zap.Error(err))
			writeError(writer, http.StatusBadRequest, "offset must be a non-negative integer")
			return
		}
		page, err := events.Events(request.Context(), models.EventSearch{
			Provider: strings.TrimSpace(request.URL.Query().Get("provider")),
			RunID:    strings.TrimSpace(request.URL.Query().Get("runId")),
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

func eventQueryInt(request *http.Request, name string, fallback int) (int, error) {
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
		jobs, err := previews.Preview(request.Context(), provider, settings)
		if err != nil {
			if errors.Is(err, services.ErrUnknownDiscoveryProvider) || errors.Is(err, services.ErrInvalidDiscoverySettings) {
				logger.Warn("discovery preview rejected", zap.String("provider", provider), zap.Error(err))
				writeError(writer, http.StatusBadRequest, err.Error())
				return
			}
			logger.Error("discovery preview failed", zap.String("provider", provider), zap.Error(err))
			writeError(writer, http.StatusBadGateway, "could not fetch provider preview")
			return
		}
		if jobs == nil {
			jobs = []models.Job{}
		}

		logger.Info("discovery preview fetched", zap.String("provider", provider), zap.Int("job_count", len(jobs)))
		writeJSON(writer, http.StatusOK, map[string]any{"jobs": jobs})
	}
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

func jobMatchChatMessagesHandler(chat *services.JobMatchChat, logger *zap.Logger) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		id, err := strconv.ParseInt(request.PathValue("id"), 10, 64)
		if err != nil || id < 1 {
			logger.Warn("invalid job ID for match chat", zap.String("id", request.PathValue("id")))
			writeError(writer, http.StatusBadRequest, "job ID must be a positive integer")
			return
		}
		messages, err := chat.Messages(request.Context(), id)
		if err != nil {
			logger.Error("load job match chat messages failed", zap.Int64("id", id), zap.Error(err))
			writeError(writer, http.StatusInternalServerError, "could not load match chat")
			return
		}
		writeJSON(writer, http.StatusOK, map[string]any{"messages": messages})
	}
}

func jobMatchApplicationResumeHandler(chat *services.JobMatchChat, logger *zap.Logger) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		id, err := strconv.ParseInt(request.PathValue("id"), 10, 64)
		if err != nil || id < 1 {
			logger.Warn("invalid job ID for application resume", zap.String("id", request.PathValue("id")))
			writeError(writer, http.StatusBadRequest, "job ID must be a positive integer")
			return
		}
		resume, events, err := chat.ApplicationResume(request.Context(), id)
		if err != nil {
			logger.Error("load application resume failed", zap.Int64("id", id), zap.Error(err))
			writeError(writer, http.StatusInternalServerError, "could not load application resume")
			return
		}
		if events == nil {
			events = []models.ApplicationResumeAgentEvent{}
		}
		writeJSON(writer, http.StatusOK, map[string]any{"resume": resume, "events": events})
	}
}

func jobMatchChatReplyHandler(chat *services.JobMatchChat, logger *zap.Logger) http.HandlerFunc {
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
		message, err := chat.Reply(request.Context(), id, body.Content, body.RequestID)
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
		if err != nil {
			logger.Error("reply to job match chat failed", zap.Int64("id", id), zap.Error(err))
			writeError(writer, http.StatusInternalServerError, "could not reply to match chat")
			return
		}
		writeJSON(writer, http.StatusCreated, map[string]any{"message": message})
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
		messageID, err := strconv.ParseInt(request.PathValue("messageID"), 10, 64)
		if err != nil || messageID < 1 {
			logger.Warn("invalid message ID for match chat revert", zap.Int64("job_id", jobID), zap.String("message_id", request.PathValue("messageID")))
			writeError(writer, http.StatusBadRequest, "message ID must be a positive integer")
			return
		}
		if err := chat.Revert(request.Context(), jobID, messageID); err != nil {
			if errors.Is(err, services.ErrJobMatchChatUserMessageNotFound) {
				logger.Warn("user chat message not found for revert", zap.Int64("job_id", jobID), zap.Int64("message_id", messageID), zap.Error(err))
				writeError(writer, http.StatusNotFound, err.Error())
				return
			}
			logger.Error("revert job match chat failed", zap.Int64("job_id", jobID), zap.Int64("message_id", messageID), zap.Error(err))
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
			return nil
		},
		OnStop: server.http.Shutdown,
	})
}
