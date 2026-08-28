# CV Input Fixtures

These are plain-text input fixtures for profile-extraction and assessment
tests. They deliberately vary in seniority, speciality, geography, employment
preferences, evidence quality, and formatting.

The fixtures were authored for this repository after reviewing these public
examples:

- https://github.com/markdownresume/markdown-resume-templates
- https://huggingface.co/datasets/sumin-yu/PopResume

The first source is MIT-licensed. Its complete license is in
`LICENSE.markdownresume`. The second is a synthetic-resume benchmark released
under CC BY 4.0.

The following original fixtures are fictional and were authored for this
repository:

- `senior-go-backend-eu.txt`
- `product-frontend-us.txt`
- `data-platform-uk.txt`

The following fixtures are ASCII-normalized copies of named templates from
`markdownresume/markdown-resume-templates` at their source URLs. Their names,
employers, contact details, and accomplishments are template content, not
verified candidate data:

- `source-fullstack-developer.md`:
  https://github.com/markdownresume/markdown-resume-templates/blob/main/templates/fullstack-developer.md
- `source-devops-engineer.md`:
  https://github.com/markdownresume/markdown-resume-templates/blob/main/templates/devops-engineer.md

Use these files only as model input. Do not use their names, contact details,
or employers as production candidate data.

## Analyzer Snapshots

The live CV analyzer test writes one JSON result beside each input under
`profiles/results/`. Each result records the input SHA-256, analyzer version,
prompt version, resolved model, analysis timestamp, and extracted draft. The
analyzer uses OpenRouter's free-model router, which selects a compatible free
model for each request.

Normal Go tests do not call an LLM. To intentionally refresh snapshots after a
prompt, schema, or model change, run:

```sh
go test ./internal/services -run TestAnalyzeCVFixtures -args -update-cv-results
```

The test reads the configured OpenRouter key through the application config.
It does not print the key, CV text, prompt, or model response.
