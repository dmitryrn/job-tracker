package services

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"html/template"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"

	"nice/internal/models"
)

var ErrResumeNotFound = errors.New("base resume not found")

type ResumePDFService struct {
	resume *ResumeService
}

func NewResumePDFService(resume *ResumeService) *ResumePDFService {
	return &ResumePDFService{resume: resume}
}

func (service *ResumePDFService) Generate(ctx context.Context) ([]byte, error) {
	resume, err := service.resume.Resume(ctx)
	if err != nil {
		return nil, fmt.Errorf("load base resume: %w", err)
	}
	if resume == nil {
		return nil, ErrResumeNotFound
	}

	data := resumePDFData{Resume: *resume}
	if resume.HasPhoto {
		photo, err := service.resume.Photo(ctx)
		if err != nil {
			return nil, fmt.Errorf("load base resume photo: %w", err)
		}
		data.Photo = template.URL("data:" + photo.ContentType + ";base64," + base64.StdEncoding.EncodeToString(photo.Data))
	}
	html, err := renderResumePDF(data)
	if err != nil {
		return nil, fmt.Errorf("render base resume HTML: %w", err)
	}

	renderCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	allocatorCtx, cancelAllocator := chromedp.NewExecAllocator(renderCtx, chromedp.DefaultExecAllocatorOptions[:]...)
	defer cancelAllocator()
	browserCtx, cancelBrowser := chromedp.NewContext(allocatorCtx)
	defer cancelBrowser()

	var output []byte
	err = chromedp.Run(browserCtx,
		chromedp.Navigate("about:blank"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			frame, err := page.GetFrameTree().Do(ctx)
			if err != nil {
				return err
			}
			return page.SetDocumentContent(frame.Frame.ID, html).Do(ctx)
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			output, _, err = page.PrintToPDF().WithPreferCSSPageSize(true).WithPrintBackground(true).Do(ctx)
			return err
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("print base resume PDF: %w", err)
	}
	return output, nil
}

type resumePDFData struct {
	Resume models.Resume
	Photo  template.URL
}

func renderResumePDF(data resumePDFData) (string, error) {
	var output bytes.Buffer
	if err := resumePDFTemplate.Execute(&output, data); err != nil {
		return "", err
	}
	return output.String(), nil
}

var resumePDFTemplate = template.Must(template.New("resume").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<style>
  @page { size: A4; margin: 15mm 16mm 18mm; }
  * { box-sizing: border-box; }
  html, body { background: #fff; color: #171717; font-family: "Noto Sans", Arial, sans-serif; font-size: 10.5pt; line-height: 1.38; margin: 0; }
  main { min-height: 264mm; }
  h1, h2, h3, p { margin-top: 0; }
  h1, h2, h3, strong { font-weight: 800; }
  h1 { font-size: 34pt; letter-spacing: -.04em; line-height: 1; margin-bottom: 7mm; }
  h2 { font-size: 20pt; letter-spacing: -.025em; line-height: 1.05; margin: 12mm 0 5mm; }
  h3 { font-size: 13pt; line-height: 1.2; margin: 0 0 3mm; }
  .intro { break-inside: avoid; }
  .photo { display: block; height: 43mm; margin: 0 0 6mm; object-fit: cover; width: 43mm; }
  .headline { font-size: 18pt; font-weight: 400; letter-spacing: .01em; margin-bottom: 5mm; text-transform: uppercase; }
  .contact { font-size: 12pt; font-weight: 800; margin-bottom: 9mm; }
  .contact span + span::before, .skills span + span::before { content: " | "; padding: 0 .15em; }
  .summary { font-size: 12pt; line-height: 1.42; margin-bottom: 7mm; white-space: pre-line; }
  .links { color: #555; font-size: 9.5pt; margin-bottom: 7mm; }
  .links a { color: inherit; text-decoration: none; }
  .links span + span::before { content: " | "; padding: 0 .15em; }
  .skills { font-size: 12pt; font-weight: 800; line-height: 1.35; }
  .section { break-inside: avoid; }
  .section + .section { margin-top: 7mm; }
  ul { margin: 0; padding-left: 8mm; }
  li { margin: 0 0 2.4mm; padding-left: 1.8mm; }
  .role { margin: 0 0 9mm; }
  .role-heading, .education-heading { align-items: baseline; display: flex; gap: 4mm; justify-content: space-between; }
  .role-heading h3, .education-heading h3 { margin-bottom: 3mm; }
  .dates { flex: 0 0 auto; font-size: 11pt; font-weight: 800; text-align: right; }
  .location, .stack { color: #555; margin: 0 0 3mm; }
  .stack { font-size: 10.5pt; }
  .education { break-inside: avoid; margin-bottom: 7mm; }
  .education p { margin-bottom: 2mm; }
</style>
</head>
<body>
<main>
  <section class="intro">
    <h1>{{.Resume.FullName}}</h1>
    {{if .Photo}}<img class="photo" src="{{.Photo}}" alt="">{{end}}
    {{if .Resume.Headline}}<p class="headline">{{.Resume.Headline}}</p>{{end}}
    <p class="contact">{{if .Resume.Location}}<span>{{.Resume.Location}}</span>{{end}}{{if .Resume.Phone}}<span>{{.Resume.Phone}}</span>{{end}}{{if .Resume.Email}}<span>{{.Resume.Email}}</span>{{end}}</p>
    {{range .Resume.SummaryParagraphs}}<p class="summary">{{.}}</p>{{end}}
    {{if .Resume.Links}}<p class="links">{{range $index, $link := .Resume.Links}}{{if $index}}<span></span>{{end}}<span><a href="{{$link.URL}}">{{$link.Label}}</a></span>{{end}}</p>{{end}}
  </section>

  {{if .Resume.Skills}}<section><h2>Key Competences</h2><p class="skills">{{range .Resume.Skills}}<span>{{.Name}}</span>{{end}}</p></section>{{end}}

  {{if .Resume.Competencies}}<section><h2>Competencies</h2>{{range .Resume.Competencies}}<div class="section"><h3>{{.Title}}</h3>{{if .Bullets}}<ul>{{range .Bullets}}<li>{{.}}</li>{{end}}</ul>{{end}}</div>{{end}}</section>{{end}}

  {{if .Resume.Experience}}<section><h2>Work Experience</h2>{{range .Resume.Experience}}<article class="role"><div class="role-heading"><h3>{{.Company}}{{if .Title}} | {{.Title}}{{end}}</h3><span class="dates">{{.StartDate}}{{if .IsCurrent}} - present{{else if .EndDate}} - {{.EndDate}}{{end}}</span></div>{{if .Location}}<p class="location">{{.Location}}</p>{{end}}{{if .Bullets}}<ul>{{range .Bullets}}<li>{{.}}</li>{{end}}</ul>{{end}}{{if .Stack}}<p class="stack">Stack: {{.Stack}}</p>{{end}}</article>{{end}}</section>{{end}}

  {{if .Resume.Education}}<section><h2>Education</h2>{{range .Resume.Education}}<article class="education"><div class="education-heading"><h3>{{.Institution}}{{if .Location}}, {{.Location}}{{end}}</h3><span class="dates">{{.StartDate}}{{if .EndDate}} - {{.EndDate}}{{end}}</span></div>{{if .Degree}}<p><strong>{{.Degree}}</strong>{{if .FieldOfStudy}}, {{.FieldOfStudy}}{{end}}</p>{{end}}{{if .Details}}<p>{{.Details}}</p>{{end}}</article>{{end}}</section>{{end}}
</main>
</body>
</html>`))
