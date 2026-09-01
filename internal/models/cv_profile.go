package models

type UserProfile struct {
	ID                int64                    `json:"id"`
	Headline          string                   `json:"headline"`
	Location          string                   `json:"location"`
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

type CVProfileDraft struct {
	Name              string              `json:"name"`
	Headline          string              `json:"headline"`
	Location          string              `json:"location"`
	WorkAuthorization []ProfileEvidence   `json:"workAuthorization"`
	Experience        []ProfileExperience `json:"experience"`
	Skills            []ProfileSkill      `json:"skills"`
	Constraints       []ProfileConstraint `json:"constraints"`
	Unknowns          []string            `json:"unknowns"`
}

type ProfileEvidence struct {
	Value    string `json:"value"`
	Evidence string `json:"evidence"`
}

type ProfileExperience struct {
	ID        string   `json:"id"`
	Company   string   `json:"company"`
	Title     string   `json:"title"`
	StartDate string   `json:"startDate"`
	EndDate   string   `json:"endDate"`
	Evidence  []string `json:"evidence"`
}

type ProfileSkill struct {
	Name          string   `json:"name"`
	Concept       string   `json:"concept"`
	ExperienceIDs []string `json:"experienceIds"`
	Evidence      []string `json:"evidence"`
}

type ProfileConstraint struct {
	Kind     string `json:"kind"`
	Value    string `json:"value"`
	Evidence string `json:"evidence"`
}
