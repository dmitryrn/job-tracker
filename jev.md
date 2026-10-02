# Adding Jev to an application

Use Jev where an application needs a bounded semantic decision—classification,
routing, scoring, verification, or a yes/no judgment. A candidate can come from
an existing LLM call, deterministic logic that has become hard to maintain, a
manual review step, or a new workflow that currently has no decision layer.
Keep exact rules, workflow, thresholds, side effects, and text generation in
application code or an LLM.

## 1. Understand the decision primitives

- [Primitives](https://docs.typesafe.ai/primitives.md) — choose between
  `Choice`, `Score`, and `Noul`.
- [How to build with System One](https://docs.typesafe.ai/concepts/how-to-build-with-system-one.md)
  — structure State, decompose questions, batch independent judgments, and
  compose answers in code.
- [Confidence](https://docs.typesafe.ai/confidence.md) — use probabilities and
  confidence to act, review, or escalate.
- [TypeSafe API](https://docs.typesafe.ai/api.md) — request and response
  contract.

## 2. Find and design a decision layer

- [jev-architect](https://github.com/karanb192/jev-architect) — agent skill
  for discovering Jev opportunities, designing question packs and control
  flow, and auditing an existing proposal.
- [jevify](https://github.com/altryne/jevify) — finds suitable decision points
  in an existing codebase and helps design typed Jev questions.
- [Chat2Jev](https://github.com/Chandler-Sun/chat2jev) — migration workbench
  for an existing LLM workflow; converts OpenAI-compatible chat requests into
  Jev State and Questions, compares the old LLM with Jev, and saves reusable
  question packs.
- [typesafe-jev-examples](https://github.com/rajivkuriakose/typesafe-jev-examples)
  — starter ticket-triage and reranking examples.

Design checklist:

1. Identify a decision currently handled by an LLM, rules, a person, or not at
   all.
2. Confirm that it needs semantic judgment rather than an exact rule or
   ordinary calculation. Keep exact, stable rules in code.
3. Put the relevant facts in structured State.
4. Ask one narrow question per independent judgment.
5. Define explicit options or levels, including a no-match option where needed.
6. Let code choose thresholds, actions, retries, and fallbacks.
7. Route uncertain or generative cases to review, deterministic handling, or
   an LLM.

## 3. Compose the decision layer in code

- [JevFlow](https://github.com/Mawfyy/jevflow) — composes `Choice`, `Score`,
  and `Noul` answers into deterministic, explainable workflows.
- [huncho](https://github.com/edgardcham/huncho) — named decisions,
  enter/exit thresholds, hysteresis, replay, and reliability calibration.
- [discern](https://github.com/doeixd/discern) — typed branches with an
  explicit `Uncertain` outcome that callers must handle.
- [jev-recipes](https://github.com/agencyenterprise/jev-recipes) — reusable
  TypeScript workflows for routing, reranking, verification, answerability,
  clarification, tool gates, completion checks, and model routing.
  - [Recipe catalog](https://jev-recipes.com/recipes/)
  - [Evaluation guide](https://jev-recipes.com/docs/evaluation/)
  - [Getting-started examples](https://github.com/agencyenterprise/jev-recipes/tree/main/examples/getting-started)

## 4. Test and improve it

- [jevcal](https://github.com/abhixhek/jevcal) — fits per-question
  confidence thresholds against labeled data, validates on held-out data,
  estimates fallback volume, and checks thresholds in CI.
- [jev-align](https://github.com/sutro-sh/jev-align) — evaluates CSV,
  JSONL, and Parquet cases, sends ambiguous cases to humans, and improves the
  saved decision definition from accepted labels.
- [jev-trust](https://pypi.org/project/jev-trust/) — records decisions and
  measures domain-specific accuracy, Brier score, ECE, and effective
  confidence.
- [Awesome Jev Robustness](https://github.com/Yifan-Lan/awesome-jev-robustness)
  — independent tests covering calibration, consistency, prompt injection,
  abstention, and failure modes.

Recommended evaluation loop:

1. Build a representative labeled fixture set.
2. Run Jev and store raw State, questions, answers, probabilities, and model
   version.
3. Measure accuracy, coverage, false positives, false negatives, latency, and
   cost at candidate thresholds.
4. Improve State, question wording, criteria, or code policy—not just the
   threshold.
5. Validate on held-out cases and add regression tests before deployment.
6. Monitor drift and re-evaluate when `jev-latest`, inputs, or policies change.

## Official implementation references

- [Python SDK](https://docs.typesafe.ai/sdk/python.md)
- [JavaScript SDK](https://docs.typesafe.ai/sdk/javascript.md)
- [Official TypeSafe agent skills](https://github.com/typesafe-ai/skills)
