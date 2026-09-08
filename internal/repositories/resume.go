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
		SELECT id, full_name, headline, town, country, email, phone,
			CASE WHEN photo_data IS NULL THEN 0 ELSE 1 END, updated_at
		FROM resumes WHERE base_resume = 1`,
	).Scan(&resume.ID, &resume.FullName, &resume.Headline, &resume.Town, &resume.Country, &resume.Email, &resume.Phone, &resume.HasPhoto, &resume.UpdatedAt)
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
		INSERT INTO resumes (id, title, base_resume, full_name, headline, town, country, email, phone, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			title = excluded.title,
			base_resume = excluded.base_resume,
			full_name = excluded.full_name,
			headline = excluded.headline,
			town = excluded.town,
			country = excluded.country,
			email = excluded.email,
			phone = excluded.phone,
			updated_at = excluded.updated_at`,
		resume.ID, "Base resume", true, resume.FullName, resume.Headline, resume.Town, resume.Country, resume.Email, resume.Phone, resume.UpdatedAt, resume.UpdatedAt,
	); err != nil {
		return models.Resume{}, fmt.Errorf("save base resume: %w", err)
	}
	if err := reconcileResumeSections(ctx, transaction, &resume); err != nil {
		return models.Resume{}, err
	}
	if err := transaction.Commit(); err != nil {
		return models.Resume{}, fmt.Errorf("commit base resume: %w", err)
	}
	stored, err := repository.Resume(ctx)
	if err != nil {
		return models.Resume{}, err
	}
	if stored == nil {
		return models.Resume{}, fmt.Errorf("load saved base resume: no resume found")
	}
	return *stored, nil
}

func reconcileResumeSections(ctx context.Context, transaction *sql.Tx, resume *models.Resume) error {
	var err error
	if resume.SummaryParagraphs, err = reconcileResumeText(ctx, transaction, "resume_summary_paragraphs", "resume_id", resume.ID, resume.SummaryParagraphs); err != nil {
		return fmt.Errorf("save base resume summary paragraphs: %w", err)
	}
	if resume.Links, err = reconcileResumeLinks(ctx, transaction, resume.ID, resume.Links); err != nil {
		return fmt.Errorf("save base resume links: %w", err)
	}
	if resume.Skills, err = reconcileResumeSkills(ctx, transaction, resume.ID, resume.Skills); err != nil {
		return fmt.Errorf("save base resume skills: %w", err)
	}
	if resume.Experience, err = reconcileResumeExperience(ctx, transaction, resume.ID, resume.Experience); err != nil {
		return fmt.Errorf("save base resume experience: %w", err)
	}
	if resume.Education, err = reconcileResumeEducation(ctx, transaction, resume.ID, resume.Education); err != nil {
		return fmt.Errorf("save base resume education: %w", err)
	}
	return nil
}

func reconcileResumeText(ctx context.Context, transaction *sql.Tx, table, parentColumn string, parentID int64, values []models.ResumeText) ([]models.ResumeText, error) {
	existing, err := sectionIDs(ctx, transaction, table, parentColumn, parentID)
	if err != nil {
		return nil, err
	}
	seen := make(map[int64]bool, len(values))
	for index := range values {
		value := &values[index]
		if value.ID == 0 {
			result, err := transaction.ExecContext(ctx, fmt.Sprintf("INSERT INTO %s (%s, content, sort_order) VALUES (?, ?, ?)", table, parentColumn), parentID, value.Content, index)
			if err != nil {
				return nil, err
			}
			value.ID, err = result.LastInsertId()
			if err != nil {
				return nil, err
			}
		} else {
			if !existing[value.ID] || seen[value.ID] {
				return nil, fmt.Errorf("invalid section ID %d", value.ID)
			}
			if _, err := transaction.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET content = ?, sort_order = ? WHERE id = ? AND %s = ?", table, parentColumn), value.Content, index, value.ID, parentID); err != nil {
				return nil, err
			}
		}
		seen[value.ID] = true
	}
	return values, deleteUnseenSections(ctx, transaction, table, existing, seen)
}

func reconcileResumeLinks(ctx context.Context, transaction *sql.Tx, resumeID int64, values []models.ResumeLink) ([]models.ResumeLink, error) {
	existing, err := sectionIDs(ctx, transaction, "resume_links", "resume_id", resumeID)
	if err != nil {
		return nil, err
	}
	seen := make(map[int64]bool, len(values))
	for index := range values {
		value := &values[index]
		if value.ID == 0 {
			result, err := transaction.ExecContext(ctx, `INSERT INTO resume_links (resume_id, label, url, sort_order) VALUES (?, ?, ?, ?)`, resumeID, value.Label, value.URL, index)
			if err != nil {
				return nil, err
			}
			value.ID, err = result.LastInsertId()
			if err != nil {
				return nil, err
			}
		} else if !existing[value.ID] || seen[value.ID] {
			return nil, fmt.Errorf("invalid resume link ID %d", value.ID)
		} else if _, err := transaction.ExecContext(ctx, `UPDATE resume_links SET label = ?, url = ?, sort_order = ? WHERE id = ? AND resume_id = ?`, value.Label, value.URL, index, value.ID, resumeID); err != nil {
			return nil, err
		}
		seen[value.ID] = true
	}
	return values, deleteUnseenSections(ctx, transaction, "resume_links", existing, seen)
}

func reconcileResumeSkills(ctx context.Context, transaction *sql.Tx, resumeID int64, values []models.ResumeSkill) ([]models.ResumeSkill, error) {
	existing, err := sectionIDs(ctx, transaction, "resume_skills", "resume_id", resumeID)
	if err != nil {
		return nil, err
	}
	seen := make(map[int64]bool, len(values))
	for index := range values {
		value := &values[index]
		if value.ID == 0 {
			result, err := transaction.ExecContext(ctx, `INSERT INTO resume_skills (resume_id, name, sort_order) VALUES (?, ?, ?)`, resumeID, value.Name, index)
			if err != nil {
				return nil, err
			}
			value.ID, err = result.LastInsertId()
			if err != nil {
				return nil, err
			}
		} else if !existing[value.ID] || seen[value.ID] {
			return nil, fmt.Errorf("invalid resume skill ID %d", value.ID)
		} else if _, err := transaction.ExecContext(ctx, `UPDATE resume_skills SET name = ?, sort_order = ? WHERE id = ? AND resume_id = ?`, value.Name, index, value.ID, resumeID); err != nil {
			return nil, err
		}
		seen[value.ID] = true
	}
	return values, deleteUnseenSections(ctx, transaction, "resume_skills", existing, seen)
}

func reconcileResumeExperience(ctx context.Context, transaction *sql.Tx, resumeID int64, values []models.ResumeExperience) ([]models.ResumeExperience, error) {
	existing, err := sectionIDs(ctx, transaction, "resume_experience", "resume_id", resumeID)
	if err != nil {
		return nil, err
	}
	seen := make(map[int64]bool, len(values))
	for index := range values {
		value := &values[index]
		if value.ID == 0 {
			result, err := transaction.ExecContext(ctx, `INSERT INTO resume_experience (resume_id, company, title, location, start_date, end_date, is_current, stack, sort_order) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, resumeID, value.Company, value.Title, value.Location, value.StartDate, value.EndDate, value.IsCurrent, value.Stack, index)
			if err != nil {
				return nil, err
			}
			value.ID, err = result.LastInsertId()
			if err != nil {
				return nil, err
			}
		} else if !existing[value.ID] || seen[value.ID] {
			return nil, fmt.Errorf("invalid resume experience ID %d", value.ID)
		} else if _, err := transaction.ExecContext(ctx, `UPDATE resume_experience SET company = ?, title = ?, location = ?, start_date = ?, end_date = ?, is_current = ?, stack = ?, sort_order = ? WHERE id = ? AND resume_id = ?`, value.Company, value.Title, value.Location, value.StartDate, value.EndDate, value.IsCurrent, value.Stack, index, value.ID, resumeID); err != nil {
			return nil, err
		}
		var bulletErr error
		value.Bullets, bulletErr = reconcileResumeText(ctx, transaction, "resume_experience_bullets", "experience_id", value.ID, value.Bullets)
		if bulletErr != nil {
			return nil, bulletErr
		}
		seen[value.ID] = true
	}
	return values, deleteUnseenSections(ctx, transaction, "resume_experience", existing, seen)
}

func reconcileResumeEducation(ctx context.Context, transaction *sql.Tx, resumeID int64, values []models.ResumeEducation) ([]models.ResumeEducation, error) {
	existing, err := sectionIDs(ctx, transaction, "resume_education", "resume_id", resumeID)
	if err != nil {
		return nil, err
	}
	seen := make(map[int64]bool, len(values))
	for index := range values {
		value := &values[index]
		if value.ID == 0 {
			result, err := transaction.ExecContext(ctx, `INSERT INTO resume_education (resume_id, institution, location, degree, field_of_study, start_date, end_date, details, sort_order) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, resumeID, value.Institution, value.Location, value.Degree, value.FieldOfStudy, value.StartDate, value.EndDate, value.Details, index)
			if err != nil {
				return nil, err
			}
			value.ID, err = result.LastInsertId()
			if err != nil {
				return nil, err
			}
		} else if !existing[value.ID] || seen[value.ID] {
			return nil, fmt.Errorf("invalid resume education ID %d", value.ID)
		} else if _, err := transaction.ExecContext(ctx, `UPDATE resume_education SET institution = ?, location = ?, degree = ?, field_of_study = ?, start_date = ?, end_date = ?, details = ?, sort_order = ? WHERE id = ? AND resume_id = ?`, value.Institution, value.Location, value.Degree, value.FieldOfStudy, value.StartDate, value.EndDate, value.Details, index, value.ID, resumeID); err != nil {
			return nil, err
		}
		seen[value.ID] = true
	}
	return values, deleteUnseenSections(ctx, transaction, "resume_education", existing, seen)
}

func sectionIDs(ctx context.Context, transaction *sql.Tx, table, parentColumn string, parentID int64) (map[int64]bool, error) {
	rows, err := transaction.QueryContext(ctx, fmt.Sprintf("SELECT id FROM %s WHERE %s = ?", table, parentColumn), parentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make(map[int64]bool)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids[id] = true
	}
	return ids, rows.Err()
}

func deleteUnseenSections(ctx context.Context, transaction *sql.Tx, table string, existing, seen map[int64]bool) error {
	for id := range existing {
		if !seen[id] {
			if _, err := transaction.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE id = ?", table), id); err != nil {
				return err
			}
		}
	}
	return nil
}

func readResumeSummaryParagraphs(ctx context.Context, database *sql.DB, resumeID int64) ([]models.ResumeText, error) {
	rows, err := database.QueryContext(ctx, `SELECT id, content FROM resume_summary_paragraphs WHERE resume_id = ? ORDER BY sort_order, id`, resumeID)
	if err != nil {
		return nil, fmt.Errorf("get base resume summary paragraphs: %w", err)
	}
	defer rows.Close()
	paragraphs := make([]models.ResumeText, 0)
	for rows.Next() {
		var paragraph models.ResumeText
		if err := rows.Scan(&paragraph.ID, &paragraph.Content); err != nil {
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
	rows, err := database.QueryContext(ctx, `SELECT id, label, url FROM resume_links WHERE resume_id = ? ORDER BY sort_order, id`, resumeID)
	if err != nil {
		return nil, fmt.Errorf("get base resume links: %w", err)
	}
	defer rows.Close()
	links := make([]models.ResumeLink, 0)
	for rows.Next() {
		var link models.ResumeLink
		if err := rows.Scan(&link.ID, &link.Label, &link.URL); err != nil {
			return nil, fmt.Errorf("scan base resume link: %w", err)
		}
		links = append(links, link)
	}
	return links, rows.Err()
}

func readResumeSkills(ctx context.Context, database *sql.DB, resumeID int64) ([]models.ResumeSkill, error) {
	rows, err := database.QueryContext(ctx, `SELECT id, name FROM resume_skills WHERE resume_id = ? ORDER BY sort_order, id`, resumeID)
	if err != nil {
		return nil, fmt.Errorf("get base resume skills: %w", err)
	}
	defer rows.Close()
	skills := make([]models.ResumeSkill, 0)
	for rows.Next() {
		var skill models.ResumeSkill
		if err := rows.Scan(&skill.ID, &skill.Name); err != nil {
			return nil, fmt.Errorf("scan base resume skill: %w", err)
		}
		skills = append(skills, skill)
	}
	return skills, rows.Err()
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
		bulletRows, err := database.QueryContext(ctx, `SELECT id, content FROM resume_experience_bullets WHERE experience_id = ? ORDER BY sort_order, id`, row.id)
		if err != nil {
			return nil, fmt.Errorf("get base resume experience bullets: %w", err)
		}
		entry := row.entry
		entry.ID = row.id
		entry.Bullets = make([]models.ResumeText, 0)
		for bulletRows.Next() {
			var bullet models.ResumeText
			if err := bulletRows.Scan(&bullet.ID, &bullet.Content); err != nil {
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
		SELECT id, institution, location, degree, field_of_study, start_date, end_date, details
		FROM resume_education WHERE resume_id = ? ORDER BY sort_order, id`, resumeID)
	if err != nil {
		return nil, fmt.Errorf("get base resume education: %w", err)
	}
	defer rows.Close()
	education := make([]models.ResumeEducation, 0)
	for rows.Next() {
		var entry models.ResumeEducation
		if err := rows.Scan(&entry.ID, &entry.Institution, &entry.Location, &entry.Degree, &entry.FieldOfStudy, &entry.StartDate, &entry.EndDate, &entry.Details); err != nil {
			return nil, fmt.Errorf("scan base resume education: %w", err)
		}
		education = append(education, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate base resume education: %w", err)
	}
	return education, nil
}
