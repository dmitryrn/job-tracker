package models

type UserProfile struct {
	ID                int64                    `json:"id"`
	Headline          string                   `json:"headline"`
	WorkAuthorization string                   `json:"workAuthorization"`
	Summary           string                   `json:"summary"`
	Skills            []UserProfileSkill       `json:"skills"`
	WorkHistory       []UserProfileWorkHistory `json:"workHistory"`
	Education         []UserProfileEducation   `json:"education"`
	UpdatedAt         string                   `json:"updatedAt"`
}

type UserProfileSkill struct {
	Name  string `json:"name"`
	Level string `json:"level"`
	Notes string `json:"notes"`
}

type UserProfileWorkHistory struct {
	Company   string `json:"company"`
	Title     string `json:"title"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
	Body      string `json:"body"`
}

type UserProfileEducation struct {
	Institution string `json:"institution"`
	Degree      string `json:"degree"`
	StartDate   string `json:"startDate"`
	EndDate     string `json:"endDate"`
	Body        string `json:"body"`
}

type Resume struct {
	ID                int64              `json:"id"`
	FullName          string             `json:"fullName"`
	Headline          string             `json:"headline"`
	Town              string             `json:"town"`
	Country           string             `json:"country"`
	Email             string             `json:"email"`
	Phone             string             `json:"phone"`
	SummaryParagraphs []ResumeText       `json:"summaryParagraphs"`
	Links             []ResumeLink       `json:"links"`
	Skills            []ResumeSkill      `json:"skills"`
	Competencies      []ResumeCompetency `json:"competencies"`
	Experience        []ResumeExperience `json:"experience"`
	Education         []ResumeEducation  `json:"education"`
	HasPhoto          bool               `json:"hasPhoto"`
	UpdatedAt         string             `json:"updatedAt"`
}

type ResumePhoto struct {
	ContentType string
	Data        []byte
}

type ResumeLink struct {
	ID    int64  `json:"id"`
	Label string `json:"label"`
	URL   string `json:"url"`
}

type ResumeText struct {
	ID      int64  `json:"id"`
	Content string `json:"content"`
}

type ResumeSkill struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type ResumeCompetency struct {
	ID      int64        `json:"id"`
	Title   string       `json:"title"`
	Bullets []ResumeText `json:"bullets"`
}

type ResumeExperience struct {
	ID        int64        `json:"id"`
	Company   string       `json:"company"`
	Title     string       `json:"title"`
	Location  string       `json:"location"`
	StartDate string       `json:"startDate"`
	EndDate   string       `json:"endDate"`
	IsCurrent bool         `json:"isCurrent"`
	Stack     string       `json:"stack"`
	Bullets   []ResumeText `json:"bullets"`
}

type ResumeEducation struct {
	ID           int64  `json:"id"`
	Institution  string `json:"institution"`
	Location     string `json:"location"`
	Degree       string `json:"degree"`
	FieldOfStudy string `json:"fieldOfStudy"`
	StartDate    string `json:"startDate"`
	EndDate      string `json:"endDate"`
	Details      string `json:"details"`
}
