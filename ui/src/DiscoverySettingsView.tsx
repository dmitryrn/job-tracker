import { useEffect, useState, type FormEvent } from "react";
import { fetchDiscoverySettings, saveDiscoverySettings, type DiscoverySettings } from "./api";

const emptySettings: DiscoverySettings = {
  adzuna: { query: "", country: "", maxDaysOld: 30, maxPages: 1, resultsPerPage: 50, workplace: "remote-hybrid" },
  remotive: { query: "", category: "software-development" },
  jobicy: { count: 50, geo: "", industry: "", tag: "" },
};

export default function DiscoverySettingsView() {
  const [settings, setSettings] = useState<DiscoverySettings>(emptySettings);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [saved, setSaved] = useState(false);

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

  function update(provider: keyof DiscoverySettings, field: string, value: string | number) {
    setSettings((current) => ({ ...current, [provider]: { ...current[provider], [field]: value } }));
    setSaved(false);
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

  return (
    <section className="profile-page">
      <header className="profile-header">
        <div>
          <p className="eyebrow">Discovery</p>
          <h1>What roles to search for</h1>
          <p>These filters apply when each provider next refreshes. Credentials and refresh schedules stay on the server.</p>
        </div>
      </header>

      {loading ? <p className="browse-loading">Loading search setup...</p> : (
        <form className="profile-form" onSubmit={(event) => void submit(event)}>
          <section className="profile-section">
            <h2>Adzuna</h2>
            <p>Germany-focused aggregated listings, filtered locally for the preferred work arrangement.</p>
            <div className="profile-fields">
              <label>Search phrase<input value={settings.adzuna.query} onChange={(event) => update("adzuna", "query", event.target.value)} /></label>
              <label>Country code<input value={settings.adzuna.country} onChange={(event) => update("adzuna", "country", event.target.value)} placeholder="de" /></label>
              <label>Posted within days<input type="number" min="1" value={settings.adzuna.maxDaysOld} onChange={(event) => update("adzuna", "maxDaysOld", Number(event.target.value))} /></label>
              <label>Pages to fetch<input type="number" min="1" value={settings.adzuna.maxPages} onChange={(event) => update("adzuna", "maxPages", Number(event.target.value))} /></label>
              <label>Results per page<input type="number" min="1" value={settings.adzuna.resultsPerPage} onChange={(event) => update("adzuna", "resultsPerPage", Number(event.target.value))} /></label>
              <label>Work arrangement<select value={settings.adzuna.workplace} onChange={(event) => update("adzuna", "workplace", event.target.value)}><option value="any">Any</option><option value="remote">Remote only</option><option value="remote-hybrid">Remote or hybrid</option></select></label>
            </div>
          </section>

          <section className="profile-section">
            <h2>Remotive</h2>
            <p>Remote listings filtered by a search phrase and Remotive category slug.</p>
            <div className="profile-fields">
              <label>Search phrase<input value={settings.remotive.query} onChange={(event) => update("remotive", "query", event.target.value)} /></label>
              <label>Category slug<input value={settings.remotive.category} onChange={(event) => update("remotive", "category", event.target.value)} placeholder="software-development" /></label>
            </div>
          </section>

          <section className="profile-section">
            <h2>Jobicy</h2>
            <p>Remote listings filtered by geography, industry, and an optional tag.</p>
            <div className="profile-fields">
              <label>Results to fetch<input type="number" min="1" max="200" value={settings.jobicy.count} onChange={(event) => update("jobicy", "count", Number(event.target.value))} /></label>
              <label>Geography<input value={settings.jobicy.geo} onChange={(event) => update("jobicy", "geo", event.target.value)} placeholder="europe" /></label>
              <label>Industry slug<input value={settings.jobicy.industry} onChange={(event) => update("jobicy", "industry", event.target.value)} placeholder="engineering" /></label>
              <label>Tag (optional)<input value={settings.jobicy.tag} onChange={(event) => update("jobicy", "tag", event.target.value)} /></label>
            </div>
          </section>

          {error && <p className="query-error">{error}</p>}
          <div className="profile-actions">
            <button className="save-profile" disabled={saving}>{saving ? "Saving..." : "Save search setup"}</button>
            {saved && <span>Saved. The next provider refresh will use these filters.</span>}
          </div>
        </form>
      )}
    </section>
  );
}
