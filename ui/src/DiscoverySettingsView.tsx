import { useEffect, useState, type FormEvent } from "react";
import { fetchDiscoverySettings, saveDiscoverySettings, streamProviderPreview, triggerDiscoverySync, type DiscoverySettings } from "./api";
import JSONTree from "./JSONTree";

const emptySettings: DiscoverySettings = {
  adzuna: { enabled: true, query: "", country: "", maxDaysOld: 30, maxPages: 1, resultsPerPage: 50, workplace: "remote-hybrid" },
  remotive: { enabled: true, query: "", category: "software-development" },
  jobicy: { enabled: true, count: 50, geo: "", industry: "", tag: "" },
  linkedin: { enabled: false, query: "", location: "Europe", postedWithin: "", workplace: "", experienceLevel: "", limit: 25 },
};

type DiscoveryProvider = keyof DiscoverySettings;

export default function DiscoverySettingsView() {
  const [settings, setSettings] = useState<DiscoverySettings>(emptySettings);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [fetching, setFetching] = useState<DiscoveryProvider>();
  const [error, setError] = useState("");
  const [saved, setSaved] = useState(false);
  const [fetchMessage, setFetchMessage] = useState("");
  const [previewing, setPreviewing] = useState<DiscoveryProvider>();
  const [previews, setPreviews] = useState<Partial<Record<DiscoveryProvider, unknown[]>>>({});
  const [previewErrors, setPreviewErrors] = useState<Partial<Record<DiscoveryProvider, string>>>({});

  useEffect(() => {
    const controller = new AbortController();
    async function load() {
      try {
        const result = await fetchDiscoverySettings(controller.signal);
        if (!controller.signal.aborted) {
          setSettings(result.settings);
          setError("");
        }
      } catch (reason) {
        if (!(reason instanceof DOMException && reason.name === "AbortError")) {
          setError(reason instanceof Error ? reason.message : "Could not load search setup");
        }
      } finally {
        if (!controller.signal.aborted) {
          setLoading(false);
        }
      }
    }
    void load();
    return () => controller.abort();
  }, []);

  function update(provider: keyof DiscoverySettings, field: string, value: string | number | boolean) {
    setSettings((current) => ({ ...current, [provider]: { ...current[provider], [field]: value } }));
    setSaved(false);
    setFetchMessage("");
  }

  async function fetchJobs(provider: DiscoveryProvider) {
    setFetching(provider);
    try {
      const savedSettings = await saveDiscoverySettings(settings);
      setSettings(savedSettings.settings);
      setSaved(true);
      const result = await triggerDiscoverySync(provider);
      setFetchMessage(result.started ? `${provider} job fetch started with this search setup.` : "A job fetch is already in progress.");
      setError("");
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "Could not start job fetch");
    } finally {
      setFetching(undefined);
    }
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSaving(true);
    try {
      const result = await saveDiscoverySettings(settings);
      setSettings(result.settings);
      setSaved(true);
      setError("");
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "Could not save search setup");
    } finally {
      setSaving(false);
    }
  }

  async function preview(provider: DiscoveryProvider) {
    setPreviewing(provider);
    setPreviewErrors((current) => ({ ...current, [provider]: "" }));
    setPreviews((current) => ({ ...current, [provider]: [] }));
    try {
      await streamProviderPreview(provider, settings, (job) => {
        setPreviews((current) => ({ ...current, [provider]: [...(current[provider] ?? []), job] }));
      });
    } catch (reason) {
      setPreviewErrors((current) => ({ ...current, [provider]: reason instanceof Error ? reason.message : "Could not fetch provider preview" }));
    } finally {
      setPreviewing(undefined);
    }
  }

  return (
    <section className="profile-page">
      <header className="profile-header">
        <div>
          <p className="eyebrow">Discovery</p>
          <h1>What roles to search for</h1>
          <p>These filters apply when each provider next refreshes. Preview a provider with the current form values without saving or importing jobs. Credentials and refresh schedules stay on the server.</p>
        </div>
      </header>

      {loading ? <p className="browse-loading">Loading search setup...</p> : (
        <form className="profile-form" onSubmit={(event) => void submit(event)}>
          <section className="profile-section">
            <div className="profile-section-heading">
              <h2>Adzuna</h2>
              <div className="provider-section-actions">
                 <label className="provider-toggle"><input type="checkbox" checked={settings.adzuna.enabled} onChange={(event) => update("adzuna", "enabled", event.target.checked)} /> Enabled</label>
                  <button className="secondary-action" type="button" disabled={previewing === "adzuna"} onClick={() => void preview("adzuna")}>{previewing === "adzuna" ? "Fetching..." : "Fetch preview"}</button>
                  <button className="secondary-action" type="button" disabled={saving || fetching !== undefined} onClick={() => void fetchJobs("adzuna")}>{fetching === "adzuna" ? "Starting..." : "Fetch jobs now"}</button>
                <PreviewOutcome jobs={previews.adzuna} fetching={previewing === "adzuna"} />
              </div>
            </div>
            {previewErrors.adzuna && <p className="query-error">{previewErrors.adzuna}</p>}
            <p>Germany-focused aggregated listings, filtered locally for the preferred work arrangement.</p>
            <div className="profile-fields">
              <label>Search phrase<input value={settings.adzuna.query} onChange={(event) => update("adzuna", "query", event.target.value)} /></label>
              <label>Country code<input value={settings.adzuna.country} onChange={(event) => update("adzuna", "country", event.target.value)} placeholder="de" /></label>
              <label>Posted within days<input type="number" min="1" value={settings.adzuna.maxDaysOld} onChange={(event) => update("adzuna", "maxDaysOld", Number(event.target.value))} /></label>
              <label>Pages to fetch<input type="number" min="1" value={settings.adzuna.maxPages} onChange={(event) => update("adzuna", "maxPages", Number(event.target.value))} /></label>
              <label>Results per page<input type="number" min="1" value={settings.adzuna.resultsPerPage} onChange={(event) => update("adzuna", "resultsPerPage", Number(event.target.value))} /></label>
              <label>Work arrangement<select value={settings.adzuna.workplace} onChange={(event) => update("adzuna", "workplace", event.target.value)}><option value="any">Any</option><option value="remote">Remote only</option><option value="remote-hybrid">Remote or hybrid</option></select></label>
            </div>
            {previews.adzuna !== undefined && <ProviderPreview jobs={previews.adzuna} fetching={previewing === "adzuna"} />}
          </section>

          <section className="profile-section">
            <div className="profile-section-heading">
              <h2>Remotive</h2>
              <div className="provider-section-actions">
                 <label className="provider-toggle"><input type="checkbox" checked={settings.remotive.enabled} onChange={(event) => update("remotive", "enabled", event.target.checked)} /> Enabled</label>
                  <button className="secondary-action" type="button" disabled={previewing === "remotive"} onClick={() => void preview("remotive")}>{previewing === "remotive" ? "Fetching..." : "Fetch preview"}</button>
                  <button className="secondary-action" type="button" disabled={saving || fetching !== undefined} onClick={() => void fetchJobs("remotive")}>{fetching === "remotive" ? "Starting..." : "Fetch jobs now"}</button>
                <PreviewOutcome jobs={previews.remotive} fetching={previewing === "remotive"} />
              </div>
            </div>
            {previewErrors.remotive && <p className="query-error">{previewErrors.remotive}</p>}
            <p>Remote listings filtered by a search phrase and Remotive category slug.</p>
            <div className="profile-fields">
              <label>Search phrase<input value={settings.remotive.query} onChange={(event) => update("remotive", "query", event.target.value)} /></label>
              <label>Category slug<input value={settings.remotive.category} onChange={(event) => update("remotive", "category", event.target.value)} placeholder="software-development" /></label>
            </div>
            {previews.remotive !== undefined && <ProviderPreview jobs={previews.remotive} fetching={previewing === "remotive"} />}
          </section>

          <section className="profile-section">
            <div className="profile-section-heading">
              <h2>Jobicy</h2>
              <div className="provider-section-actions">
                 <label className="provider-toggle"><input type="checkbox" checked={settings.jobicy.enabled} onChange={(event) => update("jobicy", "enabled", event.target.checked)} /> Enabled</label>
                  <button className="secondary-action" type="button" disabled={previewing === "jobicy"} onClick={() => void preview("jobicy")}>{previewing === "jobicy" ? "Fetching..." : "Fetch preview"}</button>
                  <button className="secondary-action" type="button" disabled={saving || fetching !== undefined} onClick={() => void fetchJobs("jobicy")}>{fetching === "jobicy" ? "Starting..." : "Fetch jobs now"}</button>
                <PreviewOutcome jobs={previews.jobicy} fetching={previewing === "jobicy"} />
              </div>
            </div>
            {previewErrors.jobicy && <p className="query-error">{previewErrors.jobicy}</p>}
            <p>Remote listings filtered by geography, industry, and an optional tag.</p>
            <div className="profile-fields">
              <label>Results to fetch<input type="number" min="1" max="200" value={settings.jobicy.count} onChange={(event) => update("jobicy", "count", Number(event.target.value))} /></label>
              <label>Geography<input value={settings.jobicy.geo} onChange={(event) => update("jobicy", "geo", event.target.value)} placeholder="europe" /></label>
              <label>Industry slug<input value={settings.jobicy.industry} onChange={(event) => update("jobicy", "industry", event.target.value)} placeholder="engineering" /></label>
              <label>Tag (optional)<input value={settings.jobicy.tag} onChange={(event) => update("jobicy", "tag", event.target.value)} /></label>
            </div>
            {previews.jobicy !== undefined && <ProviderPreview jobs={previews.jobicy} fetching={previewing === "jobicy"} />}
          </section>

          <section className="profile-section">
            <div className="profile-section-heading">
              <h2>LinkedIn</h2>
              <div className="provider-section-actions">
                 <label className="provider-toggle"><input type="checkbox" checked={settings.linkedin.enabled} onChange={(event) => update("linkedin", "enabled", event.target.checked)} /> Enabled</label>
                  <button className="secondary-action" type="button" disabled={previewing === "linkedin"} onClick={() => void preview("linkedin")}>{previewing === "linkedin" ? "Fetching..." : "Fetch preview"}</button>
                  <button className="secondary-action" type="button" disabled={saving || fetching !== undefined} onClick={() => void fetchJobs("linkedin")}>{fetching === "linkedin" ? "Starting..." : "Fetch jobs now"}</button>
                <PreviewOutcome jobs={previews.linkedin} fetching={previewing === "linkedin"} />
              </div>
            </div>
            {previewErrors.linkedin && <p className="query-error">{previewErrors.linkedin}</p>}
            <p>Public LinkedIn job listings, fetched without an account. Keep this disabled unless you want to import from LinkedIn.</p>
            <div className="profile-fields">
              <label>Search phrase<input value={settings.linkedin.query} onChange={(event) => update("linkedin", "query", event.target.value)} /></label>
              <label>Location<input value={settings.linkedin.location} onChange={(event) => update("linkedin", "location", event.target.value)} /></label>
              <label>Posted within<select value={settings.linkedin.postedWithin} onChange={(event) => update("linkedin", "postedWithin", event.target.value)}><option value="">Any time</option><option value="r86400">Past 24 hours</option><option value="r604800">Past week</option><option value="r2592000">Past month</option></select></label>
              <label>Workplace<select value={settings.linkedin.workplace} onChange={(event) => update("linkedin", "workplace", event.target.value)}><option value="">Any workplace</option><option value="1">On-site</option><option value="2">Remote</option><option value="3">Hybrid</option></select></label>
              <label>Experience level<select value={settings.linkedin.experienceLevel} onChange={(event) => update("linkedin", "experienceLevel", event.target.value)}><option value="">Any level</option><option value="1">Internship</option><option value="2">Entry level</option><option value="3">Associate</option><option value="4">Mid-Senior level</option><option value="5">Director</option><option value="6">Executive</option></select></label>
              <label>Results to fetch<input type="number" min="1" max="100" value={settings.linkedin.limit} onChange={(event) => update("linkedin", "limit", Number(event.target.value))} /></label>
            </div>
            {previews.linkedin !== undefined && <ProviderPreview jobs={previews.linkedin} fetching={previewing === "linkedin"} />}
          </section>

          {error && <p className="query-error">{error}</p>}
          <div className="profile-actions">
            <button className="save-profile" disabled={saving || fetching !== undefined}>{saving ? "Saving..." : "Save search setup"}</button>
            {saved && <span>Saved. The next provider refresh will use these filters.</span>}
            {fetchMessage && <span>{fetchMessage}</span>}
          </div>
        </form>
      )}
    </section>
  );
}

function ProviderPreview({ jobs, fetching }: { jobs: unknown[]; fetching: boolean }) {
  return (
    <section className="provider-preview" aria-live="polite">
      <div className="provider-preview-heading">
        <h3>Fetched JSON</h3>
        <span>{jobs.length} {jobs.length === 1 ? "job" : "jobs"}</span>
      </div>
      {jobs.length === 0 ? <p className="provider-preview-empty">{fetching ? "Fetching jobs..." : "The provider request succeeded, but no jobs matched these filters."}</p> : <JSONTree value={{ jobs }} />}
    </section>
  );
}

function PreviewOutcome({ jobs, fetching }: { jobs: unknown[] | undefined; fetching: boolean }) {
	if (jobs === undefined || fetching) {
		return null;
	}
	return <span className={jobs.length === 0 ? "provider-preview-outcome empty" : "provider-preview-outcome"}>{jobs.length === 0 ? "No matches" : `${jobs.length} fetched`}</span>;
}
