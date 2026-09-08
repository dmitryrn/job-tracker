package services

import (
	"strings"
	"testing"

	"nice/internal/models"
)

func TestRenderResumePDFRendersSkillsAsTagsAtTheEnd(t *testing.T) {
	t.Parallel()

	html, err := renderResumePDF(resumePDFData{Resume: models.Resume{
		Skills: []models.ResumeSkill{
			{Name: "Golang"},
			{Name: "Backend Development & REST APIs"},
		},
		Education: []models.ResumeEducation{{Institution: "Example University"}},
	}})
	if err != nil {
		t.Fatalf("renderResumePDF() error = %v", err)
	}

	skills := `<section><h2>Skills</h2><div class="skills"><span>Golang</span><span>Backend Development &amp; REST APIs</span></div></section>`
	if !strings.Contains(html, skills) {
		t.Fatalf("rendered skills = %q", html)
	}

	if strings.Index(html, skills) < strings.Index(html, "<h2>Education</h2>") {
		t.Fatalf("skills must follow education: %q", html)
	}
}
