package services

import "html/template"

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
  h2 { break-after: avoid; font-size: 20pt; letter-spacing: -.025em; line-height: 1.05; margin: 12mm 0 5mm; }
  h3 { font-size: 13pt; line-height: 1.2; margin: 0 0 3mm; }
  .intro { break-inside: avoid; }
  .photo { display: block; height: 43mm; margin: 0 0 6mm; object-fit: cover; width: 43mm; }
  .headline { font-size: 18pt; font-weight: 400; letter-spacing: .01em; margin-bottom: 5mm; text-transform: uppercase; }
  .contact { font-size: 12pt; font-weight: 800; margin-bottom: 2mm; }
  .contact span + span::before { content: " | "; padding: 0 .15em; }
  .summary { font-size: 12pt; line-height: 1.42; margin-bottom: 7mm; white-space: pre-line; }
  .links { color: #555; font-size: 10.5pt; margin-bottom: 9mm; }
  .links a { color: inherit; text-decoration: none; }
  .links span + span::before { content: " | "; padding: 0 .15em; }
  .skills { display: flex; flex-wrap: wrap; gap: 4mm; }
  .skills span { border: .25mm solid #d7dbe1; border-radius: 1.5mm; font-size: 12pt; line-height: 1.2; padding: 2.5mm 4mm; }
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
    <p class="contact">{{if .Resume.Town}}<span>{{.Resume.Town}}</span>{{end}}{{if .Resume.Country}}<span>{{.Resume.Country}}</span>{{end}}{{if .Resume.Phone}}<span>{{.Resume.Phone}}</span>{{end}}{{if .Resume.Email}}<span>{{.Resume.Email}}</span>{{end}}</p>
    {{if .Resume.Links}}<p class="links">{{range $index, $link := .Resume.Links}}{{if $index}}<span></span>{{end}}<span><a href="{{$link.URL}}">{{$link.Label}}</a></span>{{end}}</p>{{end}}
    {{range .Resume.SummaryParagraphs}}<p class="summary">{{.Content}}</p>{{end}}
  </section>

   {{if .Resume.Experience}}<section><h2>Professional Experience</h2>{{range .Resume.Experience}}<article class="role"><div class="role-heading"><h3>{{.Company}}{{if .Title}} | {{.Title}}{{end}}</h3><span class="dates">{{.StartDate}}{{if .IsCurrent}} - present{{else if .EndDate}} - {{.EndDate}}{{end}}</span></div>{{if .Location}}<p class="location">{{.Location}}</p>{{end}}{{if .Bullets}}<ul>{{range .Bullets}}<li>{{.Content}}</li>{{end}}</ul>{{end}}{{if .Stack}}<p class="stack">Stack: {{.Stack}}</p>{{end}}</article>{{end}}</section>{{end}}

   {{if .Resume.Education}}<section><h2>Education</h2>{{range .Resume.Education}}<article class="education"><div class="education-heading"><h3>{{.Institution}}{{if .Location}}, {{.Location}}{{end}}</h3><span class="dates">{{.StartDate}}{{if .EndDate}} - {{.EndDate}}{{end}}</span></div>{{if .Degree}}<p><strong>{{.Degree}}</strong>{{if .FieldOfStudy}}, {{.FieldOfStudy}}{{end}}</p>{{end}}{{if .Details}}<p>{{.Details}}</p>{{end}}</article>{{end}}</section>{{end}}

   {{if .Resume.Skills}}<section><h2>Skills</h2><div class="skills">{{range .Resume.Skills}}<span>{{.Name}}</span>{{end}}</div></section>{{end}}
</main>
</body>
</html>`))
