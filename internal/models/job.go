package models

type Job struct {
	Source         string
	SourceID       string
	SourceURL      string
	Title          string
	BodyText       string
	Company        string
	Location       string
	Workplace      string
	EmploymentType string
	SalaryMin      *int64
	SalaryMax      *int64
	PostedAt       string
	MetadataJSON   string
}

type JobSearch struct {
	Search   string
	Provider string
	Fields   []string
}

type DiscoverySettings struct {
	Adzuna   AdzunaSearchSettings   `json:"adzuna"`
	Remotive RemotiveSearchSettings `json:"remotive"`
	Jobicy   JobicySearchSettings   `json:"jobicy"`
}

type AdzunaSearchSettings struct {
	Query          string `json:"query"`
	Country        string `json:"country"`
	MaxDaysOld     int    `json:"maxDaysOld"`
	MaxPages       int    `json:"maxPages"`
	ResultsPerPage int    `json:"resultsPerPage"`
	Workplace      string `json:"workplace"`
}

type RemotiveSearchSettings struct {
	Query    string `json:"query"`
	Category string `json:"category"`
}

type JobicySearchSettings struct {
	Count    int    `json:"count"`
	Geo      string `json:"geo"`
	Industry string `json:"industry"`
	Tag      string `json:"tag"`
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
