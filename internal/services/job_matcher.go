package services

import (
	"sort"
	"strings"
	"time"

	"nice/internal/models"
)

const JobMatcherVersion = "v1"

// JobMatchInput contains only the processed job and reviewed profile needed for
// an assessment. AsOf makes calculations involving a current role reproducible.
type JobMatchInput struct {
	Job     models.JobAnalysisDraft
	Profile models.CVProfileDraft
	AsOf    time.Time
}

type JobMatch struct {
	MatcherVersion    string             `json:"matcherVersion"`
	Eligibility       MatchEligibility   `json:"eligibility"`
	Requirements      []RequirementMatch `json:"requirements"`
	ScreeningFit      string             `json:"screeningFit"`
	MainRisks         []string           `json:"mainRisks"`
	RecommendedAction string             `json:"recommendedAction"`
	Confidence        string             `json:"confidence"`
}

type MatchEligibility struct {
	State  string             `json:"state"`
	Checks []EligibilityCheck `json:"checks"`
}

type EligibilityCheck struct {
	Concept         string   `json:"concept"`
	Result          string   `json:"result"`
	JobQuote        string   `json:"jobQuote"`
	ProfileEvidence []string `json:"profileEvidence"`
	Reason          string   `json:"reason"`
}

type RequirementMatch struct {
	RequirementID   string   `json:"requirementId"`
	Concept         string   `json:"concept"`
	Kind            string   `json:"kind"`
	Result          string   `json:"result"`
	Basis           string   `json:"basis"`
	JobQuote        string   `json:"jobQuote"`
	ProfileEvidence []string `json:"profileEvidence"`
	ExperienceYears *float64 `json:"experienceYears"`
	Reason          string   `json:"reason"`
}

type JobMatcher struct{}

func NewJobMatcher() *JobMatcher {
	return &JobMatcher{}
}

func (matcher *JobMatcher) Match(input JobMatchInput) JobMatch {
	checks := matchEligibility(input.Job.Constraints, input.Profile)
	requirements := make([]RequirementMatch, 0, len(input.Job.Requirements))
	for _, requirement := range input.Job.Requirements {
		requirements = append(requirements, matchRequirement(requirement, input.Profile, input.AsOf))
	}

	return buildJobMatch(checks, requirements)
}

func matchEligibility(constraints []models.JobConstraint, profile models.CVProfileDraft) []EligibilityCheck {
	checks := make([]EligibilityCheck, 0, len(constraints))
	for _, constraint := range constraints {
		check := EligibilityCheck{
			Concept:  constraint.Kind,
			Result:   "unknown",
			JobQuote: constraint.Quote,
			Reason:   "The profile does not establish whether this constraint is compatible.",
		}

		profileConstraints := constraintsForKind(profile.Constraints, constraint.Kind)
		if constraint.Kind == "work_authorization" {
			profileConstraints = append(profileConstraints, profile.WorkAuthorization...)
		}
		for _, candidateConstraint := range profileConstraints {
			result, comparable := compareConstraint(constraint.Kind, constraint.Value, candidateConstraint.Value)
			if !comparable {
				continue
			}
			check.ProfileEvidence = append(check.ProfileEvidence, candidateConstraint.Evidence)
			if result == "pass" {
				check.Result = "pass"
				check.Reason = "The job constraint matches explicit profile evidence."
				break
			}
			check.Result = "fail"
			check.Reason = "The job constraint conflicts with explicit profile evidence."
		}
		checks = append(checks, check)
	}
	return checks
}

func constraintsForKind(constraints []models.ProfileConstraint, kind string) []models.ProfileEvidence {
	matched := make([]models.ProfileEvidence, 0)
	for _, constraint := range constraints {
		if constraint.Kind == kind {
			matched = append(matched, models.ProfileEvidence{Value: constraint.Value, Evidence: constraint.Evidence})
		}
	}
	return matched
}

func compareConstraint(kind, jobValue, profileValue string) (string, bool) {
	jobValue = canonicalConstraintValue(jobValue)
	profileValue = canonicalConstraintValue(profileValue)
	if jobValue == "" || profileValue == "" {
		return "", false
	}

	switch kind {
	case "workplace", "employment_type":
		if jobValue == profileValue {
			return "pass", true
		}
		if knownConstraintValue(kind, jobValue) && knownConstraintValue(kind, profileValue) {
			return "fail", true
		}
	case "on_call", "relocation", "travel":
		if jobValue == profileValue {
			return "pass", true
		}
		if knownBooleanValue(jobValue) && knownBooleanValue(profileValue) {
			return "fail", true
		}
	case "location", "work_authorization":
		if jobValue == profileValue {
			return "pass", true
		}
	}
	return "", false
}

func canonicalConstraintValue(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.Join(strings.Fields(value), " ")
	value = strings.ReplaceAll(value, "-", "_")
	switch value {
	case "on site", "on_site":
		return "onsite"
	case "full time", "full_time":
		return "full_time"
	case "yes", "required", "true":
		return "yes"
	case "no", "not required", "false":
		return "no"
	default:
		return value
	}
}

func knownConstraintValue(kind, value string) bool {
	switch kind {
	case "workplace":
		return value == "remote" || value == "hybrid" || value == "onsite"
	case "employment_type":
		return value == "full_time" || value == "part_time" || value == "contract" || value == "permanent"
	default:
		return false
	}
}

func knownBooleanValue(value string) bool {
	return value == "yes" || value == "no"
}

func matchRequirement(requirement models.JobRequirement, profile models.CVProfileDraft, asOf time.Time) RequirementMatch {
	match := RequirementMatch{
		RequirementID: requirement.ID,
		Concept:       canonicalConcept(requirement.Concept),
		Kind:          requirement.Kind,
		Result:        "unknown",
		Basis:         "deterministic",
		JobQuote:      requirement.Quote,
	}
	if requirement.Kind == "unknown" {
		match.Basis = "unassessable_requirement"
		match.Reason = "The job analysis does not establish a concrete requirement."
		return match
	}

	skills := skillsForConcept(profile.Skills, match.Concept)
	if len(skills) == 0 {
		match.Reason = "The profile has no direct evidence for this concept."
		return match
	}
	match.ProfileEvidence = skillEvidence(skills)
	if requirement.MinimumYears == nil {
		match.Result = "met"
		match.Basis = "exact_concept"
		match.Reason = "The profile has direct evidence for the required concept."
		return match
	}

	if years := experienceYears(skills, profile.Experience, asOf); years != nil {
		match.ExperienceYears = years
		if *years >= float64(*requirement.MinimumYears) {
			match.Result = "met"
			match.Basis = "exact_concept_and_duration"
			match.Reason = "The profile has direct evidence for the concept and establishes the required duration."
			return match
		}
		match.Result = "partial"
		match.Basis = "exact_concept_insufficient_duration"
		match.Reason = "The profile has direct evidence for the concept, but establishes less than the required duration."
		return match
	}

	match.Result = "partial"
	match.Basis = "exact_concept_unverified_duration"
	match.Reason = "The profile has direct evidence for the concept, but does not establish the required duration."
	return match
}

func skillsForConcept(skills []models.ProfileSkill, concept string) []models.ProfileSkill {
	matched := make([]models.ProfileSkill, 0)
	for _, skill := range skills {
		skillConcept := skill.Concept
		if skillConcept == "" {
			skillConcept = skill.Name
		}
		if canonicalConcept(skillConcept) == concept {
			matched = append(matched, skill)
		}
	}
	return matched
}

func skillEvidence(skills []models.ProfileSkill) []string {
	seen := make(map[string]bool)
	evidence := make([]string, 0)
	for _, skill := range skills {
		for _, quote := range skill.Evidence {
			if !seen[quote] {
				seen[quote] = true
				evidence = append(evidence, quote)
			}
		}
	}
	return evidence
}

func experienceYears(skills []models.ProfileSkill, experience []models.ProfileExperience, asOf time.Time) *float64 {
	experienceByID := make(map[string]models.ProfileExperience, len(experience))
	for _, role := range experience {
		experienceByID[role.ID] = role
	}
	seenExperienceIDs := make(map[string]bool)
	intervals := make([]monthInterval, 0)
	for _, skill := range skills {
		for _, experienceID := range skill.ExperienceIDs {
			if seenExperienceIDs[experienceID] {
				continue
			}
			role, ok := experienceByID[experienceID]
			if !ok {
				continue
			}
			seenExperienceIDs[experienceID] = true
			interval, ok := parseExperienceInterval(role.StartDate, role.EndDate, asOf)
			if ok {
				intervals = append(intervals, interval)
			}
		}
	}
	if len(intervals) == 0 {
		return nil
	}

	sort.Slice(intervals, func(i, j int) bool { return intervals[i].start < intervals[j].start })
	merged := []monthInterval{intervals[0]}
	for _, interval := range intervals[1:] {
		last := &merged[len(merged)-1]
		if interval.start <= last.end {
			if interval.end > last.end {
				last.end = interval.end
			}
			continue
		}
		merged = append(merged, interval)
	}

	months := 0
	for _, interval := range merged {
		months += interval.end - interval.start
	}
	years := float64(months) / 12
	return &years
}

type monthInterval struct {
	start int
	end   int
}

func parseExperienceInterval(start, end string, asOf time.Time) (monthInterval, bool) {
	startDate, ok := parseExperienceDate(start)
	if !ok {
		return monthInterval{}, false
	}
	if strings.EqualFold(strings.TrimSpace(end), "present") {
		if asOf.IsZero() {
			return monthInterval{}, false
		}
		startMonth := startDate.Year()*12 + int(startDate.Month())
		endMonth := asOf.Year()*12 + int(asOf.Month())
		if endMonth <= startMonth {
			return monthInterval{}, false
		}
		return monthInterval{start: startMonth, end: endMonth}, true
	}
	endDate, ok := parseExperienceDate(end)
	if !ok {
		return monthInterval{}, false
	}

	startMonth := startDate.Year()*12 + int(startDate.Month())
	endMonth := endDate.Year()*12 + int(endDate.Month()) + 1
	if endMonth <= startMonth {
		return monthInterval{}, false
	}
	return monthInterval{start: startMonth, end: endMonth}, true
}

func parseExperienceDate(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	for _, layout := range []string{"Jan 2006", "January 2006", "2006-01", "2006"} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func buildJobMatch(checks []EligibilityCheck, requirements []RequirementMatch) JobMatch {
	match := JobMatch{
		MatcherVersion: JobMatcherVersion,
		Eligibility:    MatchEligibility{State: "pass", Checks: checks},
		Requirements:   requirements,
	}
	for _, check := range checks {
		if check.Result == "fail" {
			match.Eligibility.State = "fail"
			match.ScreeningFit = "mismatch"
			match.MainRisks = append(match.MainRisks, check.Reason)
			match.RecommendedAction = "skip"
			match.Confidence = "high"
			return match
		}
		if check.Result == "unknown" {
			match.Eligibility.State = "unknown"
		}
	}

	mustHave := 0
	met := 0
	partial := 0
	unknown := 0
	for _, requirement := range requirements {
		if requirement.Kind != "must_have" {
			continue
		}
		mustHave++
		switch requirement.Result {
		case "met":
			met++
		case "partial":
			partial++
		default:
			unknown++
		}
	}

	if mustHave == 0 {
		match.ScreeningFit = "unknown"
		match.MainRisks = []string{"The posting states no assessable must-have requirements."}
		match.RecommendedAction = "investigate"
		match.Confidence = "low"
		return finalizeEligibility(match)
	}
	if unknown > 0 {
		match.ScreeningFit = "weak"
		match.RecommendedAction = "investigate"
		match.Confidence = "low"
		for _, requirement := range requirements {
			if requirement.Kind == "must_have" && requirement.Result == "unknown" {
				match.MainRisks = append(match.MainRisks, requirement.Reason)
			}
		}
		return finalizeEligibility(match)
	}
	if partial > 0 {
		match.ScreeningFit = "plausible"
		match.RecommendedAction = "apply"
		match.Confidence = "medium"
		for _, requirement := range requirements {
			if requirement.Kind == "must_have" && requirement.Result == "partial" {
				match.MainRisks = append(match.MainRisks, requirement.Reason)
			}
		}
		return finalizeEligibility(match)
	}
	if met == mustHave {
		match.ScreeningFit = "strong"
		match.RecommendedAction = "apply"
		match.Confidence = "high"
		return finalizeEligibility(match)
	}

	match.ScreeningFit = "unknown"
	match.RecommendedAction = "investigate"
	match.Confidence = "low"
	return finalizeEligibility(match)
}

func finalizeEligibility(match JobMatch) JobMatch {
	if match.Eligibility.State != "unknown" || match.RecommendedAction == "skip" {
		return match
	}
	match.MainRisks = append(match.MainRisks, "At least one eligibility constraint is unknown.")
	match.RecommendedAction = "investigate"
	if match.Confidence == "high" {
		match.Confidence = "medium"
	}
	return match
}
