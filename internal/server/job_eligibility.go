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

type jobEligibilityQuestion struct {
	ID           string
	Text         string
	Positive     bool
	Criteria     map[string]string
	CollapseWhen *jobEligibilityConditionGroup
}

type jobEligibilityCondition struct {
	QuestionID string `json:"questionId"`
	Value      bool   `json:"value"`
}

type jobEligibilityConditionOperator string

const (
	OperatorOr  jobEligibilityConditionOperator = "or"
	OperatorAnd jobEligibilityConditionOperator = "and"
)

type jobEligibilityConditionGroup struct {
	Operator   jobEligibilityConditionOperator `json:"operator"`
	Conditions []jobEligibilityCondition       `json:"conditions"`
}

type jobEligibilityAnswerResult struct {
	ID           string                        `json:"id"`
	Question     string                        `json:"question"`
	Answer       string                        `json:"answer"`
	Noul         float64                       `json:"noul"`
	Positive     bool                          `json:"positive"`
	CollapseWhen *jobEligibilityConditionGroup `json:"collapseWhen,omitempty"`
}

type jobEligibilityCheck struct {
	JobID     int64                        `json:"jobId"`
	Model     string                       `json:"model"`
	CheckedAt string                       `json:"checkedAt"`
	Answers   []jobEligibilityAnswerResult `json:"answers"`
}

var jobEligibilityQuestions = []jobEligibilityQuestion{
	{
		ID:       "european_union_job_rights",
		Text:     "Does this job require European Union job rights?",
		Positive: false,
		Criteria: map[string]string{
			"true":  "The job explicitly or clearly requires EU citizenship, EU work authorization, or the legal right to work in the European Union.",
			"false": "The job does not require EU job rights, or the posting provides no indication of such a requirement.",
		},
	},
	{
		ID:       "specific_country_residence",
		Text:     "Does this job require the applicant to reside in a specific country?",
		Positive: false,
		Criteria: map[string]string{
			"true":  "The job explicitly requires residence in one named country, or says applicants must be based there.",
			"false": "The job does not require residence in one specific country, or no such requirement is stated.",
		},
	},
	{
		ID:       "visa_sponsorship",
		Text:     "Does this job provide visa sponsorship?",
		Positive: true,
		Criteria: map[string]string{
			"true":  "The job explicitly or clearly offers visa or work-permit sponsorship to applicants.",
			"false": "The job does not offer visa or work-permit sponsorship, or no such sponsorship is stated.",
		},
		CollapseWhen: &jobEligibilityConditionGroup{
			Operator: OperatorAnd,
			Conditions: []jobEligibilityCondition{
				{QuestionID: "european_union_job_rights", Value: false},
				{QuestionID: "specific_country_residence", Value: false},
			},
		},
	},
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
		response, err := client.SystemOne(request.Context(), state, jobEligibilityRequestQuestions())
		if err != nil {
			logger.Error("run TypeSafe job eligibility check failed", zap.Int64("job_id", id), zap.Error(err))
			writeError(writer, http.StatusInternalServerError, "could not run job eligibility check")
			return
		}

		answers := make([]jobEligibilityAnswerResult, 0, len(jobEligibilityQuestions))
		for _, question := range jobEligibilityQuestions {
			answer, err := jobEligibilityAnswer(response, question)
			if err != nil {
				logger.Error("decode job eligibility answer failed", zap.Int64("job_id", id), zap.String("question_id", question.ID), zap.Error(err))
				writeError(writer, http.StatusInternalServerError, "could not decode job eligibility check")
				return
			}

			answers = append(answers, answer)
		}

		writeJSON(writer, http.StatusOK, map[string]any{"eligibility": jobEligibilityCheck{
			JobID: id, Model: response.Model, CheckedAt: time.Now().UTC().Format(time.RFC3339), Answers: answers,
		}})
	}
}

func jobEligibilityAnswer(response typesafe.Response, question jobEligibilityQuestion) (jobEligibilityAnswerResult, error) {
	answer, found := response.Answers[question.ID]
	if !found || answer.Type != "noul" {
		return jobEligibilityAnswerResult{}, fmt.Errorf("TypeSafe response did not contain a %s noul answer", question.ID)
	}

	if math.IsNaN(answer.Noul) || math.IsInf(answer.Noul, 0) || answer.Noul < 0 || answer.Noul > 1 {
		return jobEligibilityAnswerResult{}, fmt.Errorf("TypeSafe %s noul %.2f is outside the supported 0 to 1 range", question.ID, answer.Noul)
	}

	value := answer.Noul >= 0.5
	choice := "no"
	if value {
		choice = "yes"
	}

	return jobEligibilityAnswerResult{
		ID: question.ID, Question: question.Text, Answer: choice, Noul: answer.Noul,
		Positive: value == question.Positive, CollapseWhen: question.CollapseWhen,
	}, nil
}

func jobEligibilityRequestQuestions() map[string]typesafe.Question {
	questions := make(map[string]typesafe.Question, len(jobEligibilityQuestions))
	for _, question := range jobEligibilityQuestions {
		questions[question.ID] = typesafe.Question{Type: "noul", Instructions: question.Text, Criteria: question.Criteria}
	}

	return questions
}
