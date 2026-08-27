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
