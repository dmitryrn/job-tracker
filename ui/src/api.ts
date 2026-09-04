export type BrowseJob = {
  id: number;
  source: string;
  sourceURL: string;
  title: string;
  company: string;
  location: string;
  workplace: string;
  employmentType: string;
  salaryMin: number | null;
  salaryMax: number | null;
  postedAt: string;
  bodyText: string;
  hasMatch: boolean;
};

export type BrowseCompany = {
  id: number;
  name: string;
  jobCount: number;
  lastSeenAt: string;
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
  location: string;
  workAuthorization: string;
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

export type ResumeCompetency = {
  id: number;
  title: string;
  bullets: ResumeText[];
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
  location: string;
  email: string;
  phone: string;
  summaryParagraphs: ResumeText[];
  links: ResumeLink[];
  skills: ResumeSkill[];
  competencies: ResumeCompetency[];
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
  linkedin: {
    enabled: boolean;
    query: string;
    location: string;
    limit: number;
  };
};

export type JobMatch = {
  jobId: number;
  content: string;
  assessment: JobMatchAssessment | null;
  createdAt: string;
};

export type JobMatchChatMessage = {
  id: number;
  jobId: number;
  role: "user" | "assistant";
  content: string;
  createdAt: string;
};

export type ApplicationResumeRevision = {
  id: number;
  revisionNumber: number;
  triggerMessageId: number;
  assistantMessageId: number;
  resume: Resume;
  summary: string;
  createdAt: string;
};

export type ApplicationResume = {
  id: number;
  jobId: number;
  rootMessageId: number;
  base: Resume;
  revisions: ApplicationResumeRevision[];
  createdAt: string;
};

export type ApplicationResumeAgentEvent = {
  id: number;
  triggerMessageId: number;
  revisionId: number;
  type: string;
  detail: string;
  createdAt: string;
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

function apiURL(path: string) {
  const configured = import.meta.env.VITE_API_URL;
  const base = configured || `${window.location.protocol}//${window.location.hostname}:4001/api`;
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

export function fetchJobs(search: string, provider: string, fields: string[], signal: AbortSignal) {
  const parameters = new URLSearchParams();
  if (search.trim()) {
    parameters.set("search", search.trim());
  }
  if (provider) {
    parameters.set("provider", provider);
  }
  parameters.set("fields", fields.join(","));
  return request<{ jobs: BrowseJob[] }>(`jobs?${parameters}`, { signal });
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

export function fetchEvents(provider: string, runId: string, limit: number, offset: number, signal: AbortSignal) {
  const parameters = new URLSearchParams({ limit: String(limit), offset: String(offset) });
  if (provider) {
    parameters.set("provider", provider);
  }
  if (runId.trim()) {
    parameters.set("runId", runId.trim());
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

export function fetchProviderPreview(provider: keyof DiscoverySettings, settings: DiscoverySettings) {
  return request<{ jobs: unknown[] }>(`discovery-preview/${provider}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(settings),
  });
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

export function resumePhotoURL() {
  return apiURL("resume/photo");
}

export function uploadResumePhoto(photo: File) {
  const form = new FormData();
  form.append("photo", photo);
  return request<{ resume: Resume }>("resume/photo", { method: "POST", body: form });
}

export function fetchJobMatch(id: number, signal: AbortSignal) {
	return request<{ match: JobMatch | null; analysis: JobAnalysis | null }>(`jobs/${id}/match`, { signal });
}

export function fetchJobMatchChat(id: number, signal: AbortSignal) {
  return request<{ messages: JobMatchChatMessage[] }>(`jobs/${id}/match/chat`, { signal });
}

export function fetchJobMatchApplicationResume(id: number, signal: AbortSignal) {
  return request<{ resume: ApplicationResume | null; events: ApplicationResumeAgentEvent[] }>(`jobs/${id}/match/application-resume`, { signal });
}

export function sendJobMatchChatMessage(id: number, content: string, requestId: string, signal: AbortSignal) {
	return request<{ message: JobMatchChatMessage }>(`jobs/${id}/match/chat`, {
		method: "POST",
		headers: { "Content-Type": "application/json" },
		body: JSON.stringify({ content, requestId }),
		signal,
	});
}

export function fetchJobMatches(signal: AbortSignal) {
  return request<{ matches: JobMatchSummary[] }>("matches", { signal });
}

export function fetchJob(id: number, signal: AbortSignal) {
  return request<{ job: BrowseJob }>(`jobs/${id}`, { signal });
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

export async function revertJobMatchChat(id: number, messageID: number) {
  const response = await fetch(apiURL(`jobs/${id}/match/chat/${messageID}`), { method: "DELETE" });
  if (!response.ok) {
    const body = await response.json().catch(() => undefined) as { error?: string } | undefined;
    throw new Error(body?.error || `Could not revert match chat (${response.status})`);
  }
}
