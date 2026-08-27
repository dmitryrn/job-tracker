# Job Sources

This project should use documented APIs and published employer job-board
endpoints. Keep request rates low, retain each job's source URL, and follow
the applicable API terms.

## Discovery Sources

### Adzuna

- Coverage: multi-country aggregate job search, including European markets.
- Authentication: requires an `app_id` and `app_key` on every request.
- Register for credentials: <https://developer.adzuna.com/signup>
- API documentation: <https://developer.adzuna.com/docs/search>
- Useful fields: job ID, title, company name, location, salary range,
  employment/contract type, date, category, description snippet, and redirect
  URL.
- Limitation: search results provide only a description snippet, so it is best
  for discovering roles and companies rather than full-text review.

### Remotive

- Coverage: global remote jobs; filter geography locally using the candidate
  location restriction.
- Authentication: none for the public API.
- API documentation: <https://github.com/remotive-com/remote-jobs-api>
- Useful fields: ID, title, company, full HTML description, category, job type,
  publication date, salary, candidate location restriction, and listing URL.
- Terms note: attribute Remotive and link to its listing when displaying its
  jobs; poll no more than a few times per day.

## Employer Job Boards

These are not global search APIs. Use them after a discovery source identifies
an employer, then add the employer's board identifier to the local watchlist.

### Greenhouse Job Board API

- Authentication: none for published job GET endpoints.
- Documentation: <https://developers.greenhouse.io/job-board.html>
- Endpoint: `https://boards-api.greenhouse.io/v1/boards/{board_token}/jobs?content=true`
- Useful fields: job ID, title, full HTML description, location, department,
  office, update date, and source/apply URL.

### Lever Postings API

- Authentication: none for published job GET endpoints.
- Documentation: <https://github.com/lever/postings-api>
- Endpoint: `https://api.lever.co/v0/postings/{site}?mode=json`
- Useful fields: job ID, title, HTML and plain-text descriptions, location,
  team, department, workplace type, salary when present, and hosted/apply URLs.

### Ashby Job Postings API

- Authentication: none for published job GET endpoints.
- Documentation: <https://developers.ashbyhq.com/docs/public-job-posting-api>
- Endpoint: `https://api.ashbyhq.com/posting-api/job-board/{job_board_name}`
- Useful fields: title, HTML and plain-text descriptions, location, address,
  remote/workplace type, department, team, employment type, publication date,
  compensation when enabled, and apply URL.

### SmartRecruiters Posting API

- Documentation: <https://developers.smartrecruiters.com/docs/endpoints>
- Endpoint: `https://api.smartrecruiters.com/v1/companies/{company_identifier}/postings`
- Useful fields: title, company, location, department, employment type,
  experience level, full job-ad sections, release date, and apply URL.
- Note: public examples are available without credentials, but current platform
  documentation also describes API-key and OAuth access for customers. Treat
  this as an employer-specific integration and observe the platform terms.

## Optional U.S. Source

### USAJOBS

- Coverage: U.S. federal jobs only.
- Authentication: registration/API key required.
- Documentation: <https://developer.usajobs.gov/APIReference>
- Useful fields: announcement text, salary, grade, locations, dates, hiring
  paths, eligibility, and application URL.
