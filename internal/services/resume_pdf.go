package services

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"html/template"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"

	"nice/internal/models"
)

type ResumePDFService struct {
	resume *ResumeService
}

type resumePDFData struct {
	Resume models.Resume
	Photo  template.URL
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

	return service.GenerateResume(ctx, *resume)
}

func (service *ResumePDFService) GenerateResume(ctx context.Context, resume models.Resume) ([]byte, error) {
	data := resumePDFData{Resume: resume}
	if resume.HasPhoto {
		photo, err := service.resume.Photo(ctx)
		if err != nil {
			return nil, fmt.Errorf("load resume photo: %w", err)
		}

		data.Photo = template.URL("data:" + photo.ContentType + ";base64," + base64.StdEncoding.EncodeToString(photo.Data)) // #nosec G203 -- the content is an internally stored image encoded as a data URL.
	}

	html, err := renderResumePDF(data)
	if err != nil {
		return nil, fmt.Errorf("render resume HTML: %w", err)
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
		return nil, fmt.Errorf("print resume PDF: %w", err)
	}

	return output, nil
}

func renderResumePDF(data resumePDFData) (string, error) {
	var output bytes.Buffer
	if err := resumePDFTemplate.Execute(&output, data); err != nil {
		return "", err
	}

	return output.String(), nil
}
