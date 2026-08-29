package models

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
