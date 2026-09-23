export type BrowseJob = {
  id: number;
  source: string;
  sourceURL: string;
  title: string;
  company: string;
  location: string;
  workplace: string;
  workplaceClassificationJSON: string | null;
  employmentType: string;
  salaryMin: number | null;
  salaryMax: number | null;
  postedAt: string;
  bodyText: string;
  lastViewedAt: string;
  hasMatch: boolean;
  profileMatchScore: number | null;
};

export type CustomJob = {
  sourceURL: string;
};

export type BrowseCompany = {
  id: number;
  name: string;
  jobCount: number;
  lastSeenAt: string;
};

export type JobPage = {
  jobs: BrowseJob[];
  total: number;
};

export type UserProfileSkill = {
  name: string;
  level: string;
  notes: string;
};

export type UserProfileWorkHistory = {
  company: string;
  title: string;
  startDate: string;
  endDate: string;
  body: string;
};

export type UserProfileEducation = {
  institution: string;
  degree: string;
  startDate: string;
  endDate: string;
  body: string;
};

export type UserProfile = {
  id: number;
  headline: string;
  workAuthorization: string;
  githubURL: string;
  linkedinURL: string;
  summary: string;
  skills: UserProfileSkill[];
  workHistory: UserProfileWorkHistory[];
  education: UserProfileEducation[];
  updatedAt: string;
};

export type ResumeLink = {
  id: number;
  label: string;
  url: string;
};

export type ResumeText = {
  id: number;
  content: string;
};

export type ResumeSkill = {
  id: number;
  name: string;
};

export type ResumeExperience = {
  id: number;
  company: string;
  title: string;
  location: string;
  startDate: string;
  endDate: string;
  isCurrent: boolean;
  stack: string;
  bullets: ResumeText[];
};

export type ResumeEducation = {
  id: number;
  institution: string;
  location: string;
  degree: string;
  fieldOfStudy: string;
  startDate: string;
  endDate: string;
  details: string;
};

export type Resume = {
  id: number;
  fullName: string;
  headline: string;
  town: string;
  country: string;
  email: string;
  phone: string;
  summaryParagraphs: ResumeText[];
  links: ResumeLink[];
  skills: ResumeSkill[];
  experience: ResumeExperience[];
  education: ResumeEducation[];
  hasPhoto: boolean;
  updatedAt: string;
};

export type DiscoverySettings = {
  adzuna: {
    enabled: boolean;
    query: string;
    country: string;
    maxDaysOld: number;
    maxPages: number;
    resultsPerPage: number;
    workplace: string;
  };
  remotive: {
    enabled: boolean;
    query: string;
    category: string;
  };
  jobicy: {
    enabled: boolean;
    count: number;
    geo: string;
    industry: string;
    tag: string;
  };
  linkedin: Array<{
    id: number;
    name: string;
    enabled: boolean;
    sortOrder: number;
    query: string;
    location: string;
    postedWithin: string;
    workplace: string;
    experienceLevel: string;
    limit: number;
  }>;
};

export type JobMatch = {
  jobId: number;
  content: string;
  assessment: JobMatchAssessment | null;
  createdAt: string;
};

export type Application = {
  jobId: number;
  appliedAt: string;
};

export type ApplicationSummary = {
  job: BrowseJob;
  appliedAt: string;
};

export type ApplicationPage = {
  applications: ApplicationSummary[];
  total: number;
};

export type JobEligibilityAnswer = {
  id: string;
  question: string;
  answer: "yes" | "no";
  noul: number;
  positive: boolean;
  collapseWhen?: {
    operator: "or" | "and";
    conditions: Array<{
      questionId: string;
      value: boolean;
    }>;
  };
};

export type JobEligibilityCheck = {
  jobId: number;
  model: string;
  checkedAt: string;
  answers: JobEligibilityAnswer[];
};

export type JobMatchChatItem = {
	jobId: number;
	sequence: number;
	type: string;
	payload: unknown;
	createdAt: string;
	requestId?: string;
};

export type JobMatchAssessment = {
  matcherVersion: string;
  model: string;
  score: number;
  label: string;
  summary: string;
  strengths: string[];
  gaps: string[];
  questions: string[];
  applicationAngle: string;
};

export type JobMatchSummary = {
	job: BrowseJob;
	createdAt: string;
	score: number;
	label: string;
	applied: boolean;
};

export type JobMatchPage = {
	matches: JobMatchSummary[];
	total: number;
};

export type AppEvent = {
  id: number;
  occurredAt: string;
  provider: string;
  runId: string;
  type: string;
  level: "info" | "error";
  message: string;
  data: Record<string, unknown>;
};

export type EventPage = {
  events: AppEvent[];
  total: number;
};

export type JobAnalysis = {
  jobId: number;
  analyzerVersion: string;
  promptVersion: string;
  inputSHA256: string;
  model: string;
  analyzedAt: string;
  normalizedDescription: string;
  analysis: {
    role: { family: string; seniority: string; seniorityConfidence: string };
    constraints: Array<{ kind: string; value: string; quote: string; confidence: string }>;
    requirements: Array<{ id: string; kind: string; concept: string; minimumYears: number | null; screeningRisk: string; quote: string; confidence: string }>;
    responsibilities: Array<{ concept: string; quote: string }>;
    preferences: Array<{ concept: string; quote: string }>;
    unknowns: string[];
  };
};

export function apiURL(path: string) {
  const configured = import.meta.env.VITE_API_URL;
  const base = configured || `${window.location.origin}/api`;
  return `${base.replace(/\/$/, "")}/${path}`;
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(apiURL(path), init);
  if (!response.ok) {
    const body = await response.json().catch(() => undefined) as { error?: string } | undefined;
    throw new Error(body?.error || `Request failed (${response.status})`);
  }
  return response.json() as Promise<T>;
}

export function fetchJobs(search: string, provider: string, match: string, fields: string[], limit: number, offset: number, signal: AbortSignal) {
  const parameters = new URLSearchParams();
  if (search.trim()) {
    parameters.set("search", search.trim());
  }
  if (provider) {
    parameters.set("provider", provider);
  }
  if (match !== "all") {
    parameters.set("match", match);
  }
  parameters.set("fields", fields.join(","));
  parameters.set("limit", String(limit));
  parameters.set("offset", String(offset));
  return request<JobPage>(`jobs?${parameters}`, { signal });
}

export function fetchProviders(signal: AbortSignal) {
  return request<{ providers: string[] }>("providers", { signal });
}

export function fetchCompanies(search: string, signal: AbortSignal) {
  const parameters = new URLSearchParams();
  if (search.trim()) {
    parameters.set("search", search.trim());
  }
  return request<{ companies: BrowseCompany[] }>(`companies?${parameters}`, { signal });
}

export function fetchEvents(provider: string, runId: string, type: string, level: string, limit: number, offset: number, signal: AbortSignal) {
  const parameters = new URLSearchParams({ limit: String(limit), offset: String(offset) });
  if (provider) {
    parameters.set("provider", provider);
  }
  if (runId.trim()) {
    parameters.set("runId", runId.trim());
  }
	if (type.trim()) {
		parameters.set("type", type.trim());
	}
	if (level) {
		parameters.set("level", level);
	}
	return request<EventPage>(`events?${parameters}`, { signal });
}

export function fetchProfile(signal: AbortSignal) {
  return request<{ profile: UserProfile | null }>("profile", { signal });
}

export function fetchDiscoverySettings(signal: AbortSignal) {
  return request<{ settings: DiscoverySettings }>("discovery-settings", { signal });
}

export function saveDiscoverySettings(settings: DiscoverySettings) {
	return request<{ settings: DiscoverySettings }>("discovery-settings", {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(settings),
	});
}

export function triggerDiscoverySync(provider: keyof DiscoverySettings) {
	return request<{ started: boolean }>(`sync/${provider}`, { method: "POST" });
}

export async function streamProviderPreview(provider: keyof DiscoverySettings, settings: DiscoverySettings, onJob: (job: unknown) => void) {
  const response = await fetch(apiURL(`discovery-preview/${provider}`), {
    method: "POST",
    headers: { Accept: "text/event-stream", "Content-Type": "application/json" },
    body: JSON.stringify(settings),
  });
  if (!response.ok) {
    const body = await response.json().catch(() => undefined) as { error?: string } | undefined;
    throw new Error(body?.error || `Request failed (${response.status})`);
  }
  if (!response.headers.get("Content-Type")?.startsWith("text/event-stream")) {
    throw new Error("Provider preview streaming is not enabled on the server");
  }
  if (!response.body) {
    throw new Error("Provider preview stream is unavailable");
  }

  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  for (;;) {
    const { done, value } = await reader.read();
    buffer += decoder.decode(value, { stream: !done });
    const events = buffer.split("\n\n");
    buffer = events.pop() ?? "";
    for (const event of events) {
      const type = event.match(/^event: (.+)$/m)?.[1];
      const data = event.match(/^data: (.+)$/m)?.[1];
      if (!type || !data) {
        continue;
      }
      const payload = JSON.parse(data) as unknown;
      if (type === "job") {
        onJob(payload);
      }
      if (type === "error") {
        const message = (payload as { error?: string }).error;
        throw new Error(message || "Could not fetch provider preview");
      }
    }
    if (done) {
      return;
    }
  }
}

export function saveProfile(profile: UserProfile) {
  return request<{ profile: UserProfile }>("profile", {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(profile),
  });
}

export function fetchResume(signal: AbortSignal) {
  return request<{ resume: Resume | null }>("resume", { signal });
}

export function saveResume(resume: Resume) {
  return request<{ resume: Resume }>("resume", {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(resume),
  });
}

export function resumePDFURL() {
  return apiURL("resume.pdf");
}

export function jobApplicationResumePDFURL(id: number) {
  return apiURL(`jobs/${id}/match/resume.pdf`);
}

export function jobApplicationCoverLetterTXTURL(id: number) {
  return apiURL(`jobs/${id}/match/cover-letter.txt`);
}

export function resumePhotoURL() {
  return apiURL("resume/photo");
}

export function uploadResumePhoto(photo: File) {
  const form = new FormData();
  form.append("photo", photo);
  return request<{ resume: Resume }>("resume/photo", { method: "POST", body: form });
}

export function fetchJobMatch(id: number, signal: AbortSignal) {
	return request<{ match: JobMatch | null; analysis: JobAnalysis | null; application: Application | null }>(`jobs/${id}/match`, { signal });
}

export function applyToJob(id: number) {
  return request<{ application: Application }>(`jobs/${id}/application`, { method: "POST" });
}

export async function unapplyFromJob(id: number) {
  const response = await fetch(apiURL(`jobs/${id}/application`), { method: "DELETE" });
  if (!response.ok) {
    const body = await response.json().catch(() => undefined) as { error?: string } | undefined;
    throw new Error(body?.error || `Could not mark job unapplied (${response.status})`);
  }
}

export function fetchApplications(limit: number, offset: number, signal: AbortSignal) {
  return request<ApplicationPage>(`applications?limit=${limit}&offset=${offset}`, { signal });
}

export function runJobEligibilityCheck(id: number) {
  return request<{ eligibility: JobEligibilityCheck }>(`jobs/${id}/match/eligibility`, { method: "POST" });
}

export function fetchJobMatchChat(id: number, signal: AbortSignal) {
	return request<{ items: JobMatchChatItem[] }>(`jobs/${id}/match/chat`, { signal });
}

export function jobMatchChatEventsURL(id: number, after = 0) {
	return apiURL(`jobs/${id}/match/chat/events?after=${after}`);
}

export function sendJobMatchChatMessage(id: number, content: string, requestId: string) {
	return request<{ item: JobMatchChatItem }>(`jobs/${id}/match/chat`, {
		method: "POST",
		headers: { "Content-Type": "application/json" },
		body: JSON.stringify({ content, requestId }),
	});
}

export async function stopJobMatchChat(id: number, requestId: string) {
	const response = await fetch(apiURL(`jobs/${id}/match/chat/${encodeURIComponent(requestId)}/stop`), { method: "POST" });
	if (!response.ok && response.status !== 404) {
		const body = await response.json().catch(() => undefined) as { error?: string } | undefined;
		throw new Error(body?.error || `Could not stop match chat (${response.status})`);
	}
}

export function fetchJobMatches(minimumScore: number | null, sort: string, limit: number, offset: number, signal: AbortSignal, viewed = "all", applied = "all") {
	const parameters = new URLSearchParams({ sort, limit: String(limit), offset: String(offset) });
	if (minimumScore !== null) {
		parameters.set("minimumScore", String(minimumScore));
	}
	if (viewed !== "all") {
		parameters.set("viewed", viewed);
	}
	if (applied !== "all") {
		parameters.set("applied", applied);
	}
	return request<JobMatchPage>(`matches?${parameters}`, { signal });
}

export function fetchJob(id: number, signal: AbortSignal) {
  return request<{ job: BrowseJob }>(`jobs/${id}`, { signal });
}

export function createCustomJob(job: CustomJob) {
  return request<{ job: BrowseJob }>("jobs", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(job),
  });
}

export function queueJobMatch(id: number, redo: boolean) {
  return request<{ queued: boolean }>(`jobs/${id}/match${redo ? "/redo" : ""}`, { method: "POST" });
}

export function queueUnmatchedJobMatches(jobIds: number[]) {
  return request<{ queued: number }>("match-queue", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ jobIds }),
  });
}

export function fetchMatchQueue(signal: AbortSignal) {
  return request<{ jobs: BrowseJob[] }>("match-queue", { signal });
}

export function reorderMatchQueue(jobIds: number[]) {
  return request<{ queued: boolean }>("match-queue", {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ jobIds }),
  });
}

export async function removeMatchQueueItem(id: number) {
  const response = await fetch(apiURL(`match-queue/${id}`), { method: "DELETE" });
  if (!response.ok) {
    const body = await response.json().catch(() => undefined) as { error?: string } | undefined;
    throw new Error(body?.error || `Could not remove job from match queue (${response.status})`);
  }
}

export async function deleteJob(id: number) {
  const response = await fetch(apiURL(`jobs/${id}`), { method: "DELETE" });
  if (!response.ok) {
    const body = await response.json().catch(() => undefined) as { error?: string } | undefined;
    throw new Error(body?.error || `Could not delete job (${response.status})`);
  }
}

export async function rejectJob(id: number, reason: string) {
  const response = await fetch(apiURL(`jobs/${id}/rejection`), {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ reason }),
  });
  if (!response.ok) {
    const body = await response.json().catch(() => undefined) as { error?: string } | undefined;
    throw new Error(body?.error || `Could not mark job as won't apply (${response.status})`);
  }
}

export async function revertJobMatchChat(id: number, sequence: number) {
	const response = await fetch(apiURL(`jobs/${id}/match/chat/${sequence}`), { method: "DELETE" });
  if (!response.ok) {
    const body = await response.json().catch(() => undefined) as { error?: string } | undefined;
    throw new Error(body?.error || `Could not revert match chat (${response.status})`);
  }
}
