package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"nice/internal/models"
)

func (repository *SQLite) Resume(ctx context.Context) (*models.Resume, error) {
	var resume models.Resume
	err := repository.db.QueryRowContext(ctx, `
		SELECT id, full_name, headline, location, email, phone,
			CASE WHEN photo_data IS NULL THEN 0 ELSE 1 END, updated_at
		FROM resumes WHERE base_resume = 1`,
	).Scan(&resume.ID, &resume.FullName, &resume.Headline, &resume.Location, &resume.Email, &resume.Phone, &resume.HasPhoto, &resume.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get base resume: %w", err)
	}

	if resume.SummaryParagraphs, err = readResumeSummaryParagraphs(ctx, repository.db, resume.ID); err != nil {
		return nil, err
	}
	if resume.Links, err = readResumeLinks(ctx, repository.db, resume.ID); err != nil {
		return nil, err
	}
	if resume.Skills, err = readResumeSkills(ctx, repository.db, resume.ID); err != nil {
		return nil, err
	}
	if resume.Competencies, err = readResumeCompetencies(ctx, repository.db, resume.ID); err != nil {
		return nil, err
	}
	if resume.Experience, err = readResumeExperience(ctx, repository.db, resume.ID); err != nil {
		return nil, err
	}
	if resume.Education, err = readResumeEducation(ctx, repository.db, resume.ID); err != nil {
		return nil, err
	}
	return &resume, nil
}

func (repository *SQLite) ResumePhoto(ctx context.Context) (*models.ResumePhoto, error) {
	var photo models.ResumePhoto
	err := repository.db.QueryRowContext(ctx, `
		SELECT photo_content_type, photo_data
		FROM resumes
		WHERE base_resume = 1 AND photo_data IS NOT NULL`,
	).Scan(&photo.ContentType, &photo.Data)
	if err != nil {
		return nil, fmt.Errorf("get base resume photo: %w", err)
	}
	return &photo, nil
}

func (repository *SQLite) SaveResumePhoto(ctx context.Context, photo models.ResumePhoto) error {
	result, err := repository.db.ExecContext(ctx, `
		UPDATE resumes
		SET photo_content_type = ?, photo_data = ?, updated_at = ?
		WHERE base_resume = 1`,
		photo.ContentType, photo.Data, time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("save base resume photo: %w", err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check base resume photo update: %w", err)
	}
	if updated == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (repository *SQLite) SaveResume(ctx context.Context, resume models.Resume) (models.Resume, error) {
	transaction, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return models.Resume{}, fmt.Errorf("begin save base resume transaction: %w", err)
	}
	defer transaction.Rollback()

	resume.ID = 1
	resume.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if _, err := transaction.ExecContext(ctx, `
		INSERT INTO resumes (id, title, base_resume, full_name, headline, location, email, phone, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			title = excluded.title,
			base_resume = excluded.base_resume,
			full_name = excluded.full_name,
			headline = excluded.headline,
			location = excluded.location,
			email = excluded.email,
			phone = excluded.phone,
			updated_at = excluded.updated_at`,
		resume.ID, "Base resume", true, resume.FullName, resume.Headline, resume.Location, resume.Email, resume.Phone, resume.UpdatedAt, resume.UpdatedAt,
	); err != nil {
		return models.Resume{}, fmt.Errorf("save base resume: %w", err)
	}
	if err := deleteResumeSections(ctx, transaction, resume.ID); err != nil {
		return models.Resume{}, err
	}
	if err := insertResumeSections(ctx, transaction, resume); err != nil {
		return models.Resume{}, err
	}
	if err := transaction.Commit(); err != nil {
		return models.Resume{}, fmt.Errorf("commit base resume: %w", err)
	}
	if err := repository.db.QueryRowContext(ctx, `SELECT photo_data IS NOT NULL FROM resumes WHERE id = ?`, resume.ID).Scan(&resume.HasPhoto); err != nil {
		return models.Resume{}, fmt.Errorf("check base resume photo: %w", err)
	}
	return resume, nil
}

func deleteResumeSections(ctx context.Context, transaction *sql.Tx, resumeID int64) error {
	statements := []string{
		`DELETE FROM resume_summary_paragraphs WHERE resume_id = ?`,
		`DELETE FROM resume_competency_bullets WHERE competency_id IN (SELECT id FROM resume_competencies WHERE resume_id = ?)`,
		`DELETE FROM resume_experience_bullets WHERE experience_id IN (SELECT id FROM resume_experience WHERE resume_id = ?)`,
		`DELETE FROM resume_competencies WHERE resume_id = ?`,
		`DELETE FROM resume_experience WHERE resume_id = ?`,
		`DELETE FROM resume_links WHERE resume_id = ?`,
		`DELETE FROM resume_skills WHERE resume_id = ?`,
		`DELETE FROM resume_education WHERE resume_id = ?`,
	}
	for _, statement := range statements {
		if _, err := transaction.ExecContext(ctx, statement, resumeID); err != nil {
			return fmt.Errorf("clear base resume sections: %w", err)
		}
	}
	return nil
}

func insertResumeSections(ctx context.Context, transaction *sql.Tx, resume models.Resume) error {
	for index, paragraph := range resume.SummaryParagraphs {
		if _, err := transaction.ExecContext(ctx, `INSERT INTO resume_summary_paragraphs (resume_id, content, sort_order) VALUES (?, ?, ?)`, resume.ID, paragraph, index); err != nil {
			return fmt.Errorf("save base resume summary paragraph: %w", err)
		}
	}
	for index, link := range resume.Links {
		if _, err := transaction.ExecContext(ctx, `INSERT INTO resume_links (resume_id, label, url, sort_order) VALUES (?, ?, ?, ?)`, resume.ID, link.Label, link.URL, index); err != nil {
			return fmt.Errorf("save base resume link: %w", err)
		}
	}
	for index, skill := range resume.Skills {
		if _, err := transaction.ExecContext(ctx, `INSERT INTO resume_skills (resume_id, name, sort_order) VALUES (?, ?, ?)`, resume.ID, skill.Name, index); err != nil {
			return fmt.Errorf("save base resume skill: %w", err)
		}
	}
	for index, competency := range resume.Competencies {
		result, err := transaction.ExecContext(ctx, `INSERT INTO resume_competencies (resume_id, title, sort_order) VALUES (?, ?, ?)`, resume.ID, competency.Title, index)
		if err != nil {
			return fmt.Errorf("save base resume competency: %w", err)
		}
		competencyID, err := result.LastInsertId()
		if err != nil {
			return fmt.Errorf("get base resume competency ID: %w", err)
		}
		for bulletIndex, bullet := range competency.Bullets {
			if _, err := transaction.ExecContext(ctx, `INSERT INTO resume_competency_bullets (competency_id, content, sort_order) VALUES (?, ?, ?)`, competencyID, bullet, bulletIndex); err != nil {
				return fmt.Errorf("save base resume competency bullet: %w", err)
			}
		}
	}
	for index, experience := range resume.Experience {
		result, err := transaction.ExecContext(ctx, `
			INSERT INTO resume_experience (resume_id, company, title, location, start_date, end_date, is_current, stack, sort_order)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			resume.ID, experience.Company, experience.Title, experience.Location, experience.StartDate, experience.EndDate, experience.IsCurrent, experience.Stack, index,
		)
		if err != nil {
			return fmt.Errorf("save base resume experience: %w", err)
		}
		experienceID, err := result.LastInsertId()
		if err != nil {
			return fmt.Errorf("get base resume experience ID: %w", err)
		}
		for bulletIndex, bullet := range experience.Bullets {
			if _, err := transaction.ExecContext(ctx, `INSERT INTO resume_experience_bullets (experience_id, content, sort_order) VALUES (?, ?, ?)`, experienceID, bullet, bulletIndex); err != nil {
				return fmt.Errorf("save base resume experience bullet: %w", err)
			}
		}
	}
	for index, education := range resume.Education {
		if _, err := transaction.ExecContext(ctx, `
			INSERT INTO resume_education (resume_id, institution, location, degree, field_of_study, start_date, end_date, details, sort_order)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			resume.ID, education.Institution, education.Location, education.Degree, education.FieldOfStudy, education.StartDate, education.EndDate, education.Details, index,
		); err != nil {
			return fmt.Errorf("save base resume education: %w", err)
		}
	}
	return nil
}

func readResumeSummaryParagraphs(ctx context.Context, database *sql.DB, resumeID int64) ([]string, error) {
	rows, err := database.QueryContext(ctx, `SELECT content FROM resume_summary_paragraphs WHERE resume_id = ? ORDER BY sort_order, id`, resumeID)
	if err != nil {
		return nil, fmt.Errorf("get base resume summary paragraphs: %w", err)
	}
	defer rows.Close()
	paragraphs := make([]string, 0)
	for rows.Next() {
		var paragraph string
		if err := rows.Scan(&paragraph); err != nil {
			return nil, fmt.Errorf("scan base resume summary paragraph: %w", err)
		}
		paragraphs = append(paragraphs, paragraph)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate base resume summary paragraphs: %w", err)
	}
	return paragraphs, nil
}

func readResumeLinks(ctx context.Context, database *sql.DB, resumeID int64) ([]models.ResumeLink, error) {
	rows, err := database.QueryContext(ctx, `SELECT label, url FROM resume_links WHERE resume_id = ? ORDER BY sort_order, id`, resumeID)
	if err != nil {
		return nil, fmt.Errorf("get base resume links: %w", err)
	}
	defer rows.Close()
	links := make([]models.ResumeLink, 0)
	for rows.Next() {
		var link models.ResumeLink
		if err := rows.Scan(&link.Label, &link.URL); err != nil {
			return nil, fmt.Errorf("scan base resume link: %w", err)
		}
		links = append(links, link)
	}
	return links, rows.Err()
}

func readResumeSkills(ctx context.Context, database *sql.DB, resumeID int64) ([]models.ResumeSkill, error) {
	rows, err := database.QueryContext(ctx, `SELECT name FROM resume_skills WHERE resume_id = ? ORDER BY sort_order, id`, resumeID)
	if err != nil {
		return nil, fmt.Errorf("get base resume skills: %w", err)
	}
	defer rows.Close()
	skills := make([]models.ResumeSkill, 0)
	for rows.Next() {
		var skill models.ResumeSkill
		if err := rows.Scan(&skill.Name); err != nil {
			return nil, fmt.Errorf("scan base resume skill: %w", err)
		}
		skills = append(skills, skill)
	}
	return skills, rows.Err()
}

func readResumeCompetencies(ctx context.Context, database *sql.DB, resumeID int64) ([]models.ResumeCompetency, error) {
	rows, err := database.QueryContext(ctx, `SELECT id, title FROM resume_competencies WHERE resume_id = ? ORDER BY sort_order, id`, resumeID)
	if err != nil {
		return nil, fmt.Errorf("get base resume competencies: %w", err)
	}
	type competencyRow struct {
		id    int64
		title string
	}
	stored := make([]competencyRow, 0)
	for rows.Next() {
		var row competencyRow
		if err := rows.Scan(&row.id, &row.title); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan base resume competency: %w", err)
		}
		stored = append(stored, row)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate base resume competencies: %w", err)
	}
	rows.Close()

	competencies := make([]models.ResumeCompetency, 0, len(stored))
	for _, row := range stored {
		bulletRows, err := database.QueryContext(ctx, `SELECT content FROM resume_competency_bullets WHERE competency_id = ? ORDER BY sort_order, id`, row.id)
		if err != nil {
			return nil, fmt.Errorf("get base resume competency bullets: %w", err)
		}
		competency := models.ResumeCompetency{Title: row.title, Bullets: make([]string, 0)}
		for bulletRows.Next() {
			var bullet string
			if err := bulletRows.Scan(&bullet); err != nil {
				bulletRows.Close()
				return nil, fmt.Errorf("scan base resume competency bullet: %w", err)
			}
			competency.Bullets = append(competency.Bullets, bullet)
		}
		if err := bulletRows.Err(); err != nil {
			bulletRows.Close()
			return nil, fmt.Errorf("iterate base resume competency bullets: %w", err)
		}
		bulletRows.Close()
		competencies = append(competencies, competency)
	}
	return competencies, nil
}

func readResumeExperience(ctx context.Context, database *sql.DB, resumeID int64) ([]models.ResumeExperience, error) {
	rows, err := database.QueryContext(ctx, `
		SELECT id, company, title, location, start_date, end_date, is_current, stack
		FROM resume_experience WHERE resume_id = ? ORDER BY sort_order, id`, resumeID)
	if err != nil {
		return nil, fmt.Errorf("get base resume experience: %w", err)
	}
	type experienceRow struct {
		id    int64
		entry models.ResumeExperience
	}
	stored := make([]experienceRow, 0)
	for rows.Next() {
		var row experienceRow
		if err := rows.Scan(&row.id, &row.entry.Company, &row.entry.Title, &row.entry.Location, &row.entry.StartDate, &row.entry.EndDate, &row.entry.IsCurrent, &row.entry.Stack); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan base resume experience: %w", err)
		}
		stored = append(stored, row)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate base resume experience: %w", err)
	}
	rows.Close()

	experience := make([]models.ResumeExperience, 0, len(stored))
	for _, row := range stored {
		bulletRows, err := database.QueryContext(ctx, `SELECT content FROM resume_experience_bullets WHERE experience_id = ? ORDER BY sort_order, id`, row.id)
		if err != nil {
			return nil, fmt.Errorf("get base resume experience bullets: %w", err)
		}
		entry := row.entry
		entry.Bullets = make([]string, 0)
		for bulletRows.Next() {
			var bullet string
			if err := bulletRows.Scan(&bullet); err != nil {
				bulletRows.Close()
				return nil, fmt.Errorf("scan base resume experience bullet: %w", err)
			}
			entry.Bullets = append(entry.Bullets, bullet)
		}
		if err := bulletRows.Err(); err != nil {
			bulletRows.Close()
			return nil, fmt.Errorf("iterate base resume experience bullets: %w", err)
		}
		bulletRows.Close()
		experience = append(experience, entry)
	}
	return experience, nil
}

func readResumeEducation(ctx context.Context, database *sql.DB, resumeID int64) ([]models.ResumeEducation, error) {
	rows, err := database.QueryContext(ctx, `
		SELECT institution, location, degree, field_of_study, start_date, end_date, details
		FROM resume_education WHERE resume_id = ? ORDER BY sort_order, id`, resumeID)
	if err != nil {
		return nil, fmt.Errorf("get base resume education: %w", err)
	}
	defer rows.Close()
	education := make([]models.ResumeEducation, 0)
	for rows.Next() {
		var entry models.ResumeEducation
		if err := rows.Scan(&entry.Institution, &entry.Location, &entry.Degree, &entry.FieldOfStudy, &entry.StartDate, &entry.EndDate, &entry.Details); err != nil {
			return nil, fmt.Errorf("scan base resume education: %w", err)
		}
		education = append(education, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate base resume education: %w", err)
	}
	return education, nil
}
