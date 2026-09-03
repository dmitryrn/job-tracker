package models

type Job struct {
	Source         string `json:"source"`
	SourceID       string `json:"sourceId"`
	SourceURL      string `json:"sourceURL"`
	Title          string `json:"title"`
	BodyText       string `json:"bodyText"`
	Company        string `json:"company"`
	Location       string `json:"location"`
	Workplace      string `json:"workplace"`
	EmploymentType string `json:"employmentType"`
	SalaryMin      *int64 `json:"salaryMin"`
	SalaryMax      *int64 `json:"salaryMax"`
	PostedAt       string `json:"postedAt"`
	MetadataJSON   string `json:"metadataJSON"`
}

type JobSearch struct {
	Search   string
	Provider string
	Fields   []string
}

type Event struct {
	ID         int64          `json:"id"`
	OccurredAt string         `json:"occurredAt"`
	Provider   string         `json:"provider"`
	RunID      string         `json:"runId"`
	Type       string         `json:"type"`
	Level      string         `json:"level"`
	Message    string         `json:"message"`
	Data       map[string]any `json:"data"`
}

type EventSearch struct {
	Provider string
	RunID    string
	Limit    int
	Offset   int
}

type EventPage struct {
	Events []Event `json:"events"`
	Total  int     `json:"total"`
}

type DiscoverySettings struct {
	Adzuna   AdzunaSearchSettings   `json:"adzuna"`
	Remotive RemotiveSearchSettings `json:"remotive"`
	Jobicy   JobicySearchSettings   `json:"jobicy"`
	LinkedIn LinkedInSearchSettings `json:"linkedin"`
}

type AdzunaSearchSettings struct {
	Enabled        bool   `json:"enabled"`
	Query          string `json:"query"`
	Country        string `json:"country"`
	MaxDaysOld     int    `json:"maxDaysOld"`
	MaxPages       int    `json:"maxPages"`
	ResultsPerPage int    `json:"resultsPerPage"`
	Workplace      string `json:"workplace"`
}

type RemotiveSearchSettings struct {
	Enabled  bool   `json:"enabled"`
	Query    string `json:"query"`
	Category string `json:"category"`
}

type JobicySearchSettings struct {
	Enabled  bool   `json:"enabled"`
	Count    int    `json:"count"`
	Geo      string `json:"geo"`
	Industry string `json:"industry"`
	Tag      string `json:"tag"`
}

type LinkedInSearchSettings struct {
	Enabled  bool   `json:"enabled"`
	Query    string `json:"query"`
	Location string `json:"location"`
	Limit    int    `json:"limit"`
}

type BrowseJob struct {
	ID             int64  `json:"id"`
	Source         string `json:"source"`
	SourceURL      string `json:"sourceURL"`
	Title          string `json:"title"`
	Company        string `json:"company"`
	Location       string `json:"location"`
	Workplace      string `json:"workplace"`
	EmploymentType string `json:"employmentType"`
	SalaryMin      *int64 `json:"salaryMin"`
	SalaryMax      *int64 `json:"salaryMax"`
	PostedAt       string `json:"postedAt"`
	BodyText       string `json:"bodyText"`
	HasMatch       bool   `json:"hasMatch"`
}

type BrowseCompany struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	JobCount   int64  `json:"jobCount"`
	LastSeenAt string `json:"lastSeenAt"`
}

type JobMatchRecord struct {
	JobID      int64               `json:"jobId"`
	Content    string              `json:"content"`
	Assessment *JobMatchAssessment `json:"assessment,omitempty"`
	CreatedAt  string              `json:"createdAt"`
}

type JobMatchChatMessage struct {
	ID        int64  `json:"id"`
	JobID     int64  `json:"jobId"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	CreatedAt string `json:"createdAt"`
}

type JobMatchAssessment struct {
	MatcherVersion   string   `json:"matcherVersion"`
	Model            string   `json:"model"`
	Score            int      `json:"score"`
	Label            string   `json:"label"`
	Summary          string   `json:"summary"`
	Strengths        []string `json:"strengths"`
	Gaps             []string `json:"gaps"`
	Questions        []string `json:"questions"`
	ApplicationAngle string   `json:"applicationAngle"`
}

type JobMatchSummary struct {
	Job       BrowseJob `json:"job"`
	CreatedAt string    `json:"createdAt"`
	Score     int       `json:"score"`
	Label     string    `json:"label"`
}
