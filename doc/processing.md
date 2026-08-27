# Job Processing and Candidate Assessment

## Purpose

This document describes how Nice can turn a raw job posting into a useful,
auditable recommendation for one candidate. The goal is to help prioritize
roles where the candidate is both interested and unlikely to be rejected for
an obvious mismatch.

The system must answer three different questions independently:

| Question | Output | Source of truth |
| --- | --- | --- |
| What does the employer appear to require? | Processed job analysis | The job posting and its source fields |
| Can this candidate credibly satisfy those requirements? | Screening-fit assessment | Confirmed candidate profile and evidence |
| Is this worth acting on now? | Apply priority | Screening fit, candidate interest, freshness, and preferences |

These must not collapse into one opaque "match score." A candidate can be
very interested in a role while having a material screening risk. That role is
a conscious long shot, not a strong recommendation.

The first version estimates **screening fit**, not the probability of an
interview. A job description cannot account for applicant competition,
recruiter behaviour, internal candidates, hiring freezes, or referrals. The
application may estimate interview likelihood only after it has meaningful
outcomes from this candidate's applications.

## Current Data

The existing `jobs` table already contains useful source facts:

- `title`, `source_url`, company, location, workplace, employment type, salary,
  and posted date.
- `body_text`, which holds plain text for some providers and HTML for others.
- `metadata_json`, which is provider-specific and may contain tags, categories,
  salary currency, level, and excerpts.

Provider metadata is supplementary rather than authoritative. It can conflict
with the visible title or description. The processor should retain it for
display and debugging, but extract requirements from the title and normalized
description.

The database may also contain repeated or closely related postings, such as a
role listed across providers or country-specific variants. Processing should
eventually cluster likely duplicates so one vacancy does not dominate the
candidate's queue.

## Design Principles

- Preserve raw source data. An analysis is derived data and can be regenerated.
- Make every material conclusion traceable to a job quote or profile evidence.
- Keep facts, inferences, preferences, and outcomes separate.
- A hard eligibility failure cannot be overridden by a high numerical score.
- Missing profile evidence is normally `unknown`, not a claim that the
  candidate lacks the skill.
- Do not treat a related technology as an exact substitute without an explicit
  taxonomy rule.
- Prefer a useful explanation and a broad fit band over an unjustified precise
  percentage.
- Version all derived analyses. A changed job description, profile, taxonomy,
  prompt, or assessment algorithm can change the result.

## Processing Pipeline

```text
provider response
    -> raw jobs row
    -> normalize description
    -> analyzeJob
    -> processed job analysis

confirmed candidate profile + processed job analysis
    -> assessJob
    -> job assessment

assessment + candidate feedback + freshness
    -> ranked candidate queue
```

`analyzeJob` and `assessJob` are separate operations with separate persisted
results. This separation prevents an LLM from silently inventing a personal
match score while it is extracting the job's requirements.

### When Processing Runs

Run job normalization and analysis after a job is inserted or updated by a
provider sync. Reanalyze only when the normalized job input, analysis schema,
prompt, or analysis taxonomy version changes. Assessment can run after an
analysis is created and whenever the candidate profile changes.

Do not block provider synchronization or job browsing on an LLM request. Queue
the derived processing work and show an `analysis pending` state until it is
complete. A failed analysis must leave the raw job browsable and retryable.

## Candidate Profile

The candidate profile is a set of confirmed claims, not merely a CV or a
free-form biography. A CV can be used to draft the profile, but the candidate
must review the extracted claims before assessments depend on them.

The profile needs four categories.

### Eligibility and Constraints

- Home country, work authorization, and countries where employment is possible.
- Remote, hybrid, onsite, relocation, and timezone preferences.
- Accepted employment types: permanent, contract, part-time, and so on.
- Salary floor and target, including currency and period where known.
- Hard no's, such as an industry, frequent travel, an on-call prohibition, or
  a relocation requirement.

### Professional Experience

- Current and target seniority.
- Total and relevant years of experience.
- Years of remote work, if this is a requirement for relevant roles.
- Roles, domains, and responsibilities: for example, B2B SaaS, platform
  engineering, product engineering, distributed systems, or people management.
- Experience that is potentially required by roles, such as customer-facing
  work, security-sensitive systems, on-call rotations, or regulated domains.

### Skills and Evidence

Each skill should include a canonical concept, not just an arbitrary string.
It should also include evidence that can be shown in an explanation.

```json
{
  "concept": "go",
  "level": "strong",
  "years": 4,
  "lastUsedAt": "2026-08",
  "evidence": [
    "Built and operated Go APIs for a multi-tenant SaaS product",
    "Designed asynchronous event processing and reliability tooling"
  ]
}
```

Good evidence describes work done and its context. A keyword listed in a CV
without supporting experience is a weak signal and should not be presented as
proof of a must-have qualification.

### Preferences

Preferences answer whether the candidate wants the role. They must not be used
as qualification evidence.

- Desired domains, company stage, mission, team size, and technical work.
- Technologies the candidate wants to use or avoid.
- Compensation target, beyond the hard salary floor.
- Candidate-entered interest rating for individual roles.

### Example Profile Shape

```json
{
  "constraints": {
    "homeCountry": "Germany",
    "workAuthorization": ["Germany", "EU"],
    "remoteRegions": ["Europe"],
    "employmentTypes": ["full-time"],
    "salaryMinimum": { "amount": 90000, "currency": "EUR", "period": "year" },
    "willingToOnCall": true
  },
  "experience": {
    "seniority": "senior",
    "remoteYears": 3,
    "domains": ["b2b_saas", "distributed_systems"]
  },
  "skills": [
    {
      "concept": "go",
      "level": "strong",
      "years": 4,
      "evidence": ["Built and operated production Go APIs"]
    }
  ],
  "preferences": {
    "interestedDomains": ["developer_tools", "b2b_saas"],
    "avoid": ["relocation"]
  }
}
```

For the initial single-candidate product, a versioned JSON profile is simpler
than prematurely normalizing every field into tables. The assessment data must
store the profile version it used. If profiles later need searching, reporting,
or multiple candidates, stable fields can be normalized without changing the
assessment contract.

## Job Normalization

Normalization makes a source job safe and consistent to analyze. It is not an
attempt to decide whether the candidate matches.

1. Preserve the original `body_text`.
2. Convert HTML descriptions to normalized plain text, retaining headings and
   list boundaries where possible.
3. Collapse irrelevant whitespace and decode HTML entities.
4. Combine normalized text with title, location, workplace, employment type,
   salary, company, and posted date in a canonical analysis input.
5. Calculate a content hash from that input.
6. Store the normalized text and hash with the analysis result.

The source fields for location, work mode, employment type, salary, and date
are structured facts. The language model may extract corroborating information
from prose, but it must not silently replace source values.

## `analyzeJob`

`analyzeJob` turns a normalized posting into an evidence-backed description of
the role. It has no access to the candidate profile and does not assign a
personal fit score.

### Inputs

- Job ID and source name.
- Title and company name.
- Normalized description text.
- Source location, workplace, employment type, salary, and posted date.
- Provider metadata, marked as supplementary.

### Outputs

The output should use a strictly validated JSON schema. The central unit is an
atomic requirement with its classification, normalized concept, source quote,
and extraction confidence.

```json
{
  "role": {
    "family": "backend_engineering",
    "seniority": "senior",
    "seniorityConfidence": "medium"
  },
  "constraints": {
    "acceptedLocations": ["Germany", "Ireland", "Spain", "Sweden", "UK"],
    "remote": true,
    "employmentTypes": ["full-time"],
    "salary": null
  },
  "requirements": [
    {
      "id": "professional-go-experience",
      "kind": "must_have",
      "concept": "go",
      "minimumYears": null,
      "screeningRisk": "high",
      "quote": "Have professional experience with Golang",
      "confidence": "high"
    },
    {
      "id": "remote-work-experience",
      "kind": "must_have",
      "concept": "remote_work",
      "minimumYears": 1,
      "screeningRisk": "high",
      "quote": "You have at least 1 year of fully remote work experience",
      "confidence": "high"
    }
  ],
  "responsibilities": [
    {
      "concept": "on_call",
      "quote": "participating in our follow-the-sun OnCall rotation"
    }
  ],
  "preferences": [],
  "unknowns": ["The posting does not state a salary"]
}
```

### Requirement Classification

The processor must distinguish these categories:

| Kind | Meaning | Assessment impact |
| --- | --- | --- |
| `must_have` | Explicit requirement or clear eligibility condition | High; an unmet requirement is a screening risk |
| `strong_preference` | Important experience, but not clearly mandatory | Medium |
| `nice_to_have` | Advantage, bonus, or preferred familiarity | Low |
| `responsibility` | Work expected after hiring | Used to assess interest and relevant evidence, not assumed to be a gate |
| `unknown` | The posting does not say enough | Does not become a negative claim |

Only explicit wording such as "required," "must," a stated minimum, or a
clear legal/location restriction should create a high-confidence must-have.
The model should return `unknown` when the text is ambiguous rather than
upgrading an implied preference into a gate.

### LLM Contract

Use an LLM API that supports schema-constrained JSON output. The extraction
prompt should instruct the model to:

- Extract only claims supported by the supplied posting.
- Include a verbatim supporting quote for every requirement and constraint.
- Classify strength conservatively.
- Leave fields unknown when the posting does not provide evidence.
- Never make claims about a candidate or their suitability.

Validate the returned JSON before persistence. Reject or mark failed outputs
that omit quotes, contain concepts outside the allowed taxonomy where one is
required, or contradict structured source fields without marking a conflict.

Store the model identifier, prompt/schema version, input hash, analysis time,
and status with the result. This makes the analysis reproducible and permits a
later prompt improvement to be rolled out deliberately.

### Taxonomy

Maintain a small, explicit taxonomy in application code. It maps known aliases
to canonical concepts:

```text
Golang -> go
Type Script -> typescript
LLM evaluation -> llm_evaluation
```

The taxonomy may also represent safe relationships, but it should be
conservative. Experience in Go may be related to backend engineering; it is
not evidence of C++ core-performance experience. TypeScript is not evidence of
professional Go experience. The assessment must expose a related-skill match
as partial, not exact.

Start with the concepts occurring in the collected jobs and extend it only when
there is a real matching need. A large universal skills ontology is unnecessary
for the first version.

## `assessJob`

`assessJob` compares one processed job analysis to one confirmed profile. It is
deterministic application logic. It may call narrowly scoped semantic helpers
to resolve approved concept aliases, but it must not ask an LLM to invent a
final score.

### Eligibility Gates

Evaluate these before calculating screening fit:

- Legal work authorization and explicitly permitted hiring locations.
- Remote, hybrid, onsite, travel, and relocation constraints.
- Accepted employment type.
- Explicit minimum compensation when currency and period can be compared.
- Explicit seniority or minimum-years requirements.
- Candidate hard no's, including on-call or industry restrictions where the
  posting clearly requires them.

Each gate produces `pass`, `fail`, or `unknown` with an explanation. A `fail`
returns `eligibility: fail` and prevents an `apply` recommendation. Missing
information should stay `unknown`; it should produce an `investigate` action,
not an automatic rejection.

### Requirement Comparison

For every extracted requirement, compare the normalized concept, level, years,
and context with profile claims and evidence. Return one of these outcomes:

| Result | Meaning |
| --- | --- |
| `met` | Profile contains direct, sufficient evidence |
| `partial` | Related or less extensive evidence exists |
| `unknown` | The profile does not establish either fit or mismatch |
| `not_met` | The profile explicitly contradicts the requirement or hard constraint |
| `not_applicable` | The requirement cannot reasonably be assessed from available data |

An assessment result must retain the job quote and the matched profile evidence.
For example, a professional Go requirement can be `met` only when the profile
contains professional Go evidence, rather than a vague adjacent technology.

### Screening Fit

Screening fit is an initial, explainable heuristic. It is useful for sorting
eligible roles; it is not an interview probability.

After eligibility passes, derive the band from:

- Coverage of explicit must-haves, weighted by screening risk.
- Seniority and relevant-years alignment.
- Directness and recency of the candidate's evidence.
- Relevant domain and responsibility experience.
- Important preferences and nice-to-haves, at lower weight.

Candidate desire must not increase screening fit. It belongs in the separate
interest signal.

The initial bands can be defined as follows:

| Band | Meaning |
| --- | --- |
| `strong` | No failed gates; almost all important must-haves have direct evidence |
| `plausible` | No failed gates; some evidence is partial or needs confirmation |
| `weak` | No failed gates, but a material must-have is unknown or only weakly related |
| `mismatch` | A gate fails or a critical must-have is explicitly not met |
| `unknown` | The job or profile has insufficient information to assess responsibly |

If a numerical implementation is useful, calculate it only within a passing
eligibility state. An initial weighting could give 60% to must-have coverage,
15% to seniority and years, 15% to direct evidence, and 10% to relevant
responsibility or domain experience. Display the band and reasons by default;
do not present the number as an objective probability.

### Assessment Shape

```json
{
  "eligibility": {
    "state": "pass",
    "checks": [
      {
        "concept": "work_location",
        "result": "pass",
        "reason": "Candidate may work in Germany; Germany is an accepted location"
      }
    ]
  },
  "screeningFit": "plausible",
  "requirements": [
    {
      "requirementId": "professional-go-experience",
      "result": "met",
      "jobQuote": "Have professional experience with Golang",
      "profileEvidence": ["Built and operated production Go APIs"]
    },
    {
      "requirementId": "remote-work-experience",
      "result": "met",
      "jobQuote": "You have at least 1 year of fully remote work experience",
      "profileEvidence": ["3 years of remote work"]
    },
    {
      "requirementId": "multi-tenancy",
      "result": "unknown",
      "jobQuote": "familiar with common distributed systems concepts (e.g., scalability, multi-tenancy, HA)",
      "profileEvidence": []
    }
  ],
  "mainRisks": ["No direct evidence of multi-tenancy or high availability work"],
  "recommendedAction": "apply",
  "confidence": "high"
}
```

## Interest, Feedback, and Ranking

Interest describes the candidate's own decision, not recruiter fit. Let the
candidate use simple controls:

- `dream`, `interested`, `maybe`, `not interested`, and `dismissed`.
- Optional reasons such as salary, domain, company, technology, or location.
- Application state: saved, researching, applied, recruiter screen, interview,
  offer, withdrawn, or rejected.

The system can learn preference similarity from these signals later. Embeddings
are appropriate for finding jobs semantically similar to saved roles, but not
for deciding whether the candidate meets a stated must-have.

### Apply Priority

Rank only roles that have not failed eligibility. The priority should combine:

- Screening-fit band and unresolved screening risks.
- Candidate's explicit interest and learned preference affinity.
- Posting freshness and whether the source is still active.
- Compensation fit, only when it can be compared accurately.
- Analysis confidence and information completeness.
- Duplicate clustering, so equivalent postings are represented once.

Suggested actions are:

| Action | Meaning |
| --- | --- |
| `apply` | Eligibility passes and screening fit is strong or plausible |
| `investigate` | A material condition is unknown, such as location, authorization, salary, or a key requirement |
| `long_shot` | Candidate is interested but there is a known material screening risk |
| `skip` | Eligibility fails, a critical requirement is explicitly not met, or candidate dismissed it |

The queue should explain its order. A useful job card leads with the action,
fit band, key matches, key risks, interest rating, and the exact requirement
quotes behind its conclusion.

## Interview Outcome Calibration

Do not label the initial assessment as "likelihood of interview." Screening
fit only estimates whether the application appears defensible against the
published requirements.

As the candidate records outcomes, retain the stage and known rejection reason:

```text
discovered -> saved/skipped -> applied -> recruiter_screen -> interview -> offer
```

After enough comparable applications, calculate observed progression rates for
segments such as screening-fit band, seniority, role family, and location. Use
smoothed estimates and broad ranges, especially with small samples. A statement
such as "6 of 18 comparable applications reached a recruiter screen" is more
honest than a false precision of "33.3% likely."

Do not surface a personalized interview-likelihood band until there are enough
outcomes to support it. Thirty or more applications may be a reasonable first
threshold, but the quality and comparability of the outcomes matters more than
the raw count.

## Persistence

The following derived records are sufficient for a first implementation:

| Record | Purpose |
| --- | --- |
| `candidate_profiles` | Versioned confirmed candidate facts and preferences |
| `job_analyses` | Normalized job text, structured extraction, input hash, prompt/schema/model version, status, and timestamps |
| `job_assessments` | Profile version plus assessment result, algorithm/taxonomy version, and timestamps |
| `job_feedback` | Manual interest, application stage, outcome, and optional reason |
| `job_clusters` later | Likely duplicate/variant postings and canonical job grouping |

An assessment should be uniquely identified by job, profile version, job
analysis version, and assessment algorithm version. It is a cacheable derived
result, not an immutable statement about the candidate.

Persist the full structured JSON initially. Add indexed columns only for fields
that are needed for filtering or reporting, such as assessment action, fit
band, profile ID, job ID, and processing status.

## API and UI Boundaries

The existing browsing endpoint can continue to serve raw jobs. Add derived data
through separate endpoints or optional response expansion rather than changing
the provider ingestion contract.

Possible operations are:

```text
GET  /api/profile
PUT  /api/profile
POST /api/jobs/{id}/analyze
GET  /api/jobs/{id}/analysis
POST /api/jobs/{id}/assess
GET  /api/jobs/{id}/assessment
PUT  /api/jobs/{id}/feedback
GET  /api/jobs?order=priority
```

Background processing should normally invoke analysis and assessment
automatically. Manual endpoints are useful for retries, profile edits, and
debugging.

The UI should not hide uncertainty. It should distinguish:

- Confirmed match: direct profile evidence supports the requirement.
- Partial match: related but incomplete evidence.
- Verify: the posting or profile does not say enough.
- Likely mismatch: an explicit hard gate or required claim conflicts.

Provide links to the original job source and expose the supporting job quote.
The candidate should be able to correct profile evidence or mark an extraction
wrong. Corrections are valuable feedback for the taxonomy and prompts.

## LLM Boundaries and Privacy

An LLM is useful for converting varied prose into a stable requirement schema.
It is not the authority on candidate qualifications, hiring likelihood, or
legal work eligibility.

- Send the minimum necessary job text to the configured LLM provider.
- Do not send the candidate profile to the job extraction call.
- Store whether external processing is enabled and make it clear to the user.
- Support a local or self-hosted model later if profile privacy requires it.
- Do not log full profile content, CVs, or model prompts containing them.
- Validate all model output as untrusted input before storing or displaying it.

The first assessment implementation does not require profile embeddings or a
general-purpose autonomous agent. A structured extraction call plus explicit
Go comparison rules is more understandable, testable, and reliable.

## Evaluation and Testing

Build a small reviewed fixture set from real job descriptions, with source data
redacted only where necessary. Each fixture should assert:

- HTML normalization preserves meaningful headings and list items.
- Explicit requirements are extracted with the correct quote and classification.
- Optional language is not promoted to a must-have.
- Location and salary source fields do not get silently overwritten.
- Skill aliases map correctly while unrelated skills do not become exact
  matches.
- A gate failure produces `mismatch` or `skip` regardless of other scores.
- Unknown information produces `investigate`, not an invented negative claim.
- Every displayed assessment reason has a job quote or profile evidence.

Keep prompt/schema fixtures separate from deterministic assessment tests.
Prompt behavior changes over time; eligibility and matching rules should have
fast, stable Go unit tests.

Review a sample of analyzed jobs before enabling automatic ranking. Measure
requirement extraction precision, requirement recall, and false hard-gate
rate. A false hard gate is particularly harmful because it can hide an
otherwise suitable opportunity.

## Incremental Delivery

### Phase 1: Explainable Screening Fit

1. Add one confirmed, versioned candidate profile.
2. Normalize job descriptions and persist the normalized text and hash.
3. Add schema-constrained job analysis with requirement quotes.
4. Implement deterministic eligibility gates and requirement comparison.
5. Display analysis, assessment, evidence, risks, and suggested action.

This phase produces value without a preference model or any claim about
interview probability.

### Phase 2: Personal Prioritization

1. Add save/skip/interest controls and reasons.
2. Add application stages and rejection reasons where known.
3. Rank by screening fit, explicit interest, freshness, and compensation.
4. Cluster obvious provider duplicates and role variants.

### Phase 3: Learned Signals

1. Use feedback to suggest jobs similar to those the candidate saved or applied
   to.
2. Calibrate broad recruiter-screen and interview progression ranges from real
   outcomes.
3. Improve the taxonomy and prompts from reviewed extraction errors.

No phase should replace the requirement-by-requirement explanation with a
black-box score.

## Open Decisions

- Whether the initial profile is entered manually, imported from a CV, or both.
- Which LLM provider or local model is used for schema-constrained extraction.
- Which skill concepts are needed for the first collected job set.
- Whether salary comparisons are limited to matching currencies initially or
  use an explicit exchange-rate source.
- How much automatic duplicate clustering is acceptable before a user review
  step is required.
- The minimum outcome history needed before showing calibrated interview bands.
