package server

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"

	"go.uber.org/zap"

	"nice/internal/clients/typesafe"
	"nice/internal/repositories"
)

type typeSafeSystemOneClient interface {
	SystemOne(context.Context, any, map[string]typesafe.Question) (typesafe.Response, error)
}

type jobEligibilityAnswerResult struct {
	Answer string  `json:"answer"`
	Noul   float64 `json:"noul"`
}

type jobEligibilityCheck struct {
	JobID                    int64                      `json:"jobId"`
	Model                    string                     `json:"model"`
	CheckedAt                string                     `json:"checkedAt"`
	EuropeanUnionJobRights   jobEligibilityAnswerResult `json:"europeanUnionJobRights"`
	SpecificCountryResidence jobEligibilityAnswerResult `json:"specificCountryResidence"`
	VisaSponsorship          jobEligibilityAnswerResult `json:"visaSponsorship"`
}

func jobEligibilityCheckHandler(jobs repositories.JobRepository, client typeSafeSystemOneClient, logger *zap.Logger) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		id, err := strconv.ParseInt(request.PathValue("id"), 10, 64)
		if err != nil || id < 1 {
			logger.Warn("invalid job ID for eligibility check", zap.String("id", request.PathValue("id")))
			writeError(writer, http.StatusBadRequest, "job ID must be a positive integer")
			return
		}
		job, err := jobs.AnalysisJob(request.Context(), id)
		if err != nil {
			logger.Error("load job for eligibility check failed", zap.Int64("job_id", id), zap.Error(err))
			writeError(writer, http.StatusInternalServerError, "could not run job eligibility check")
			return
		}
		if job == nil {
			logger.Warn("run job eligibility check for missing job", zap.Int64("job_id", id))
			writeError(writer, http.StatusNotFound, "job not found")
			return
		}

		state := map[string]any{"job": map[string]any{
			"title": job.Title, "location": job.Location, "workplace": job.Workplace,
			"employmentType": job.EmploymentType, "postedAt": job.PostedAt,
			"description": job.BodyText, "metadataJSON": job.MetadataJSON,
		}}
		response, err := client.SystemOne(request.Context(), state, map[string]typesafe.Question{
			"european_union_job_rights": {
				Type:         "noul",
				Instructions: "Does this job require European Union job rights?",
				Criteria: map[string]string{
					"true":  "The job explicitly or clearly requires EU citizenship, EU work authorization, or the legal right to work in the European Union.",
					"false": "The job does not require EU job rights, or the posting provides no indication of such a requirement.",
				},
			},
			"specific_country_residence": {
				Type:         "noul",
				Instructions: "Does this job require the applicant to reside in a specific country?",
				Criteria: map[string]string{
					"true":  "The job explicitly requires residence in one named country, or says applicants must be based there.",
					"false": "The job does not require residence in one specific country, or no such requirement is stated.",
				},
			},
			"visa_sponsorship": {
				Type:         "noul",
				Instructions: "Does this job provide visa sponsorship?",
				Criteria: map[string]string{
					"true":  "The job explicitly or clearly offers visa or work-permit sponsorship to applicants.",
					"false": "The job does not offer visa or work-permit sponsorship, or no such sponsorship is stated.",
				},
			},
		})
		if err != nil {
			logger.Error("run TypeSafe job eligibility check failed", zap.Int64("job_id", id), zap.Error(err))
			writeError(writer, http.StatusInternalServerError, "could not run job eligibility check")
			return
		}
		eu, err := jobEligibilityAnswer(response, "european_union_job_rights")
		if err != nil {
			logger.Error("decode EU job-rights eligibility answer failed", zap.Int64("job_id", id), zap.Error(err))
			writeError(writer, http.StatusInternalServerError, "could not decode job eligibility check")
			return
		}
		country, err := jobEligibilityAnswer(response, "specific_country_residence")
		if err != nil {
			logger.Error("decode country-residence eligibility answer failed", zap.Int64("job_id", id), zap.Error(err))
			writeError(writer, http.StatusInternalServerError, "could not decode job eligibility check")
			return
		}
		visaSponsorship, err := jobEligibilityAnswer(response, "visa_sponsorship")
		if err != nil {
			logger.Error("decode visa-sponsorship eligibility answer failed", zap.Int64("job_id", id), zap.Error(err))
			writeError(writer, http.StatusInternalServerError, "could not decode job eligibility check")
			return
		}
		writeJSON(writer, http.StatusOK, map[string]any{"eligibility": jobEligibilityCheck{
			JobID: id, Model: response.Model, CheckedAt: time.Now().UTC().Format(time.RFC3339),
			EuropeanUnionJobRights: eu, SpecificCountryResidence: country, VisaSponsorship: visaSponsorship,
		}})
	}
}

func jobEligibilityAnswer(response typesafe.Response, question string) (jobEligibilityAnswerResult, error) {
	answer, found := response.Answers[question]
	if !found || answer.Type != "noul" {
		return jobEligibilityAnswerResult{}, fmt.Errorf("TypeSafe response did not contain a %s noul answer", question)
	}
	if math.IsNaN(answer.Noul) || math.IsInf(answer.Noul, 0) || answer.Noul < 0 || answer.Noul > 1 {
		return jobEligibilityAnswerResult{}, fmt.Errorf("TypeSafe %s noul %.2f is outside the supported 0 to 1 range", question, answer.Noul)
	}
	choice := "no"
	if answer.Noul >= 0.5 {
		choice = "yes"
	}
	return jobEligibilityAnswerResult{Answer: choice, Noul: answer.Noul}, nil
}
