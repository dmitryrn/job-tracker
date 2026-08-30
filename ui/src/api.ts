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

export type UserProfile = {
  id: number;
  headline: string;
  location: string;
  workAuthorization: string;
  summary: string;
  skills: UserProfileSkill[];
  updatedAt: string;
};

export type JobMatch = {
  jobId: number;
  content: string;
  createdAt: string;
};

export type JobMatchSummary = {
  job: BrowseJob;
  createdAt: string;
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

export function saveProfile(profile: UserProfile) {
  return request<{ profile: UserProfile }>("profile", {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(profile),
  });
}

export function fetchJobMatch(id: number, signal: AbortSignal) {
  return request<{ match: JobMatch | null }>(`jobs/${id}/match`, { signal });
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

export async function deleteJob(id: number) {
  const response = await fetch(apiURL(`jobs/${id}`), { method: "DELETE" });
  if (!response.ok) {
    const body = await response.json().catch(() => undefined) as { error?: string } | undefined;
    throw new Error(body?.error || `Could not delete job (${response.status})`);
  }
}
