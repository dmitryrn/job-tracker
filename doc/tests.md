# Tests

The project has deterministic tests for normal development and an opt-in live
test for refreshing CV-analysis review artifacts. Run the relevant suites from
the repository root.

## Go Tests

```sh
go test ./...
```

These tests do not call external services and do not require real credentials.

## UI Tests

```sh
cd ui
npm test
npm run build
```

Vitest covers UI components and the browser-side SQLite explorer. The build
also type-checks the TypeScript application and creates the production bundle.

## CV Analyzer Tests

`internal/services/cv_analyzer_test.go` uses a fake completion client to test
versioned metadata, blank-input handling, and rejection of claims without
evidence. It is deterministic and runs with `go test ./...`.

`internal/services/cv_analyzer_live_test.go` has two distinct tests:

- `TestSavedCVAnalysesMatchFixtures` runs normally. It checks that each saved
  result in `profiles/results/` is valid JSON, references its input fixture,
  has the current analyzer and prompt versions, records a model, and has the
  correct normalized-input SHA-256.
- `TestAnalyzeCVFixtures` is skipped unless explicitly enabled. It sends every
  fixture in `profiles/` to OpenRouter and rewrites its corresponding saved
  result.

Refresh snapshots only after intentionally changing the extraction prompt,
schema, or model behavior:

```sh
go test ./internal/services -run TestAnalyzeCVFixtures -args -update-cv-results
```

The refresh reads `OPENROUTER_API_KEY` through the normal application
configuration. It never prints the key. It uses OpenRouter's free-model router,
so the resolved model and output may vary between runs. Review every changed
result before accepting it; saved results are evidence-backed review artifacts,
not deterministic golden outputs.

## Job Analyzer Tests

`internal/services/job_analyzer_test.go` uses a fake completion client to test
versioned metadata, HTML normalization, concept canonicalization, blank-input
handling, and rejection of claims without posting quotes. It is deterministic
and runs with `go test ./...`.

`internal/services/job_analyzer_live_test.go` validates saved results in
`jobs/results/` against their fixture, normalized input, schema, and hash. Its
live refresh test is skipped unless explicitly enabled:

```sh
go test ./internal/services -run TestAnalyzeJobFixtures -args -update-job-results
```
