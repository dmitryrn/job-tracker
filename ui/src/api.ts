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

export type DiscoverySettings = {
  adzuna: {
    query: string;
    country: string;
    maxDaysOld: number;
    maxPages: number;
    resultsPerPage: number;
    workplace: string;
  };
  remotive: {
    query: string;
    category: string;
  };
  jobicy: {
    count: number;
    geo: string;
    industry: string;
    tag: string;
  };
};

export type JobMatch = {
  jobId: number;
  content: string;
  assessment: JobMatchAssessment | null;
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

export function saveProfile(profile: UserProfile) {
  return request<{ profile: UserProfile }>("profile", {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(profile),
  });
}

export function fetchJobMatch(id: number, signal: AbortSignal) {
	return request<{ match: JobMatch | null; analysis: JobAnalysis | null }>(`jobs/${id}/match`, { signal });
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
