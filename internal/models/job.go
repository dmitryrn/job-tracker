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
}

type BrowseCompany struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	JobCount   int64  `json:"jobCount"`
	LastSeenAt string `json:"lastSeenAt"`
}

type JobMatchRecord struct {
	JobID     int64  `json:"jobId"`
	Content   string `json:"content"`
	CreatedAt string `json:"createdAt"`
}

type JobMatchSummary struct {
	Job       BrowseJob `json:"job"`
	CreatedAt string    `json:"createdAt"`
}
