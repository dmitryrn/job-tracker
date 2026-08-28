# Job Input Fixtures

These fixtures are fictionalized copies of job records selected from the local
SQLite database on 2026-08-28. They cover the provider formats currently stored
by the application: a short plain-text Adzuna description and HTML descriptions
from Remotive and Jobicy.

Employer names, source URLs, tracking URLs, and source-specific identifiers are
fictionalized. The role wording, source fields, and body structure otherwise
remain substantially unchanged so the fixtures can exercise normalization and
job-analysis behavior.

The fixtures are test inputs only. They are not current job listings and must
not be presented as real vacancies or attributed to real employers.

## Analyzer Snapshots

The live job-analyzer test writes one JSON result for each fixture and requested
model under `jobs/results/`. Each result records the requested and resolved model,
normalized description, input SHA-256, analyzer version, prompt version, analysis
timestamp, and evidence-backed extraction.

Normal Go tests do not call an LLM. To intentionally refresh snapshots after a
prompt, schema, or model change, run:

```sh
go test ./internal/services -run TestAnalyzeJobFixtures -args -update-job-results
```

It uses OpenRouter's free-model router by default. To review named compatible free
models, provide a comma-separated list:

```sh
go test ./internal/services -run TestAnalyzeJobFixtures -args -update-job-results -job-analyzer-models model-a:free,model-b:free
```

The test reads the configured OpenRouter key through the application config. It
does not print the key, job text, prompt, or model response.
