package models

type JobAnalysisDraft struct {
	Role             JobRole          `json:"role"`
	Constraints      []JobConstraint  `json:"constraints"`
	Requirements     []JobRequirement `json:"requirements"`
	Responsibilities []JobEvidence    `json:"responsibilities"`
	Preferences      []JobEvidence    `json:"preferences"`
	Unknowns         []string         `json:"unknowns"`
}

type JobAnalysisRecord struct {
	JobID                 int64            `json:"jobId"`
	AnalyzerVersion       string           `json:"analyzerVersion"`
	PromptVersion         string           `json:"promptVersion"`
	InputSHA256           string           `json:"inputSHA256"`
	Model                 string           `json:"model"`
	AnalyzedAt            string           `json:"analyzedAt"`
	NormalizedDescription string           `json:"normalizedDescription"`
	Analysis              JobAnalysisDraft `json:"analysis"`
}

type JobRole struct {
	Family              string `json:"family"`
	Seniority           string `json:"seniority"`
	SeniorityConfidence string `json:"seniorityConfidence"`
}

type JobConstraint struct {
	Kind       string `json:"kind"`
	Value      string `json:"value"`
	Quote      string `json:"quote"`
	Confidence string `json:"confidence"`
}

type JobRequirement struct {
	ID            string `json:"id"`
	Kind          string `json:"kind"`
	Concept       string `json:"concept"`
	MinimumYears  *int   `json:"minimumYears"`
	ScreeningRisk string `json:"screeningRisk"`
	Quote         string `json:"quote"`
	Confidence    string `json:"confidence"`
}

type JobEvidence struct {
	Concept string `json:"concept"`
	Quote   string `json:"quote"`
}
