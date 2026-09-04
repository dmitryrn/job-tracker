package services

import (
	"strings"
	"testing"

	"nice/internal/models"
)

func TestRenderResumePDFRendersSkillsAsListItems(t *testing.T) {
	t.Parallel()

	html, err := renderResumePDF(resumePDFData{Resume: models.Resume{
		Skills: []models.ResumeSkill{
			{Name: "Golang"},
			{Name: "Backend Development & REST APIs"},
		},
	}})
	if err != nil {
		t.Fatalf("renderResumePDF() error = %v", err)
	}

	if !strings.Contains(html, `<ul class="skills"><li>Golang</li><li>Backend Development &amp; REST APIs</li></ul>`) {
		t.Fatalf("rendered skills = %q", html)
	}
}
