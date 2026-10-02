package services

import (
	"bytes"
	"html/template"
	"strings"
)

var coverLetterPDFTemplate = template.Must(template.New("cover-letter").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<style>
  @page { size: A4; margin: 22mm; }
  * { box-sizing: border-box; }
  html, body { background: #fff; color: #171717; font-family: "Noto Sans", Arial, sans-serif; font-size: 11.5pt; line-height: 1.55; margin: 0; }
  main { min-height: 253mm; }
  p { margin: 0 0 7mm; }
  p:last-child { margin-bottom: 0; }
</style>
</head>
<body>
<main>
  {{range .Paragraphs}}<p>{{.}}</p>{{end}}
</main>
</body>
</html>`))

type coverLetterPDFData struct {
	Paragraphs []string
}

func renderCoverLetterPDF(content string) (string, error) {
	data := coverLetterPDFData{Paragraphs: coverLetterParagraphs(content)}
	var output bytes.Buffer
	if err := coverLetterPDFTemplate.Execute(&output, data); err != nil {
		return "", err
	}

	return output.String(), nil
}

func coverLetterParagraphs(content string) []string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.TrimSpace(content)
	if content == "" {
		return nil
	}

	return strings.Split(content, "\n\n")
}
