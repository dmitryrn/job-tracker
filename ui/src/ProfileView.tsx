import { useEffect, useState, type FormEvent } from "react";
import { fetchProfile, saveProfile, type UserProfile } from "./api";

const emptyProfile: UserProfile = {
  id: 1,
  headline: "",
  location: "",
  workAuthorization: "",
  summary: "",
  skills: [],
  workHistory: [],
  education: [],
  updatedAt: "",
};

export default function ProfileView() {
  const [profile, setProfile] = useState<UserProfile>(emptyProfile);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [savedAt, setSavedAt] = useState("");

  useEffect(() => {
    const controller = new AbortController();
    async function load() {
      try {
        const result = await fetchProfile(controller.signal);
        if (!controller.signal.aborted) {
          setProfile(result.profile ?? emptyProfile);
          setError("");
        }
      } catch (reason) {
        if (!(reason instanceof DOMException && reason.name === "AbortError")) {
          setError(reason instanceof Error ? reason.message : "Could not load profile");
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

  function updateField(field: keyof UserProfile, value: string) {
    setProfile((current) => ({ ...current, [field]: value }));
  }

  function updateSkill(index: number, field: "name" | "level" | "notes", value: string) {
    setProfile((current) => ({
      ...current,
      skills: current.skills.map((skill, skillIndex) => skillIndex === index ? { ...skill, [field]: value } : skill),
    }));
  }

  function updateWorkHistory(index: number, field: "company" | "title" | "startDate" | "endDate" | "body", value: string) {
    setProfile((current) => ({
      ...current,
      workHistory: current.workHistory.map((entry, entryIndex) => entryIndex === index ? { ...entry, [field]: value } : entry),
    }));
  }

  function updateEducation(index: number, field: "institution" | "degree" | "startDate" | "endDate" | "body", value: string) {
    setProfile((current) => ({
      ...current,
      education: current.education.map((entry, entryIndex) => entryIndex === index ? { ...entry, [field]: value } : entry),
    }));
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSaving(true);
    try {
      const result = await saveProfile(profile);
      setProfile(result.profile);
      setSavedAt(result.profile.updatedAt);
      setError("");
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "Could not save profile");
    } finally {
      setSaving(false);
    }
  }

  return (
    <section className="profile-page">
      <header className="profile-header">
        <div>
          <p className="eyebrow">Your profile</p>
          <h1>What you bring to a role</h1>
          <p>Keep this factual. The matcher will use it as the evidence source for future job assessments.</p>
        </div>
      </header>

      {loading ? <p className="browse-loading">Loading profile...</p> : (
        <form className="profile-form" onSubmit={(event) => void submit(event)}>
          <section className="profile-section">
            <h2>Basics</h2>
            <div className="profile-fields">
              <label>Professional headline<input value={profile.headline} onChange={(event) => updateField("headline", event.target.value)} placeholder="Senior backend engineer" /></label>
              <label>Location<input value={profile.location} onChange={(event) => updateField("location", event.target.value)} placeholder="Berlin, Germany" /></label>
              <label>Work authorization<input value={profile.workAuthorization} onChange={(event) => updateField("workAuthorization", event.target.value)} placeholder="Eligible to work in the EU" /></label>
            </div>
            <label className="profile-summary">Summary<textarea value={profile.summary} onChange={(event) => updateField("summary", event.target.value)} placeholder="The work, domains, and strengths you want a matcher to consider." rows={5} /></label>
          </section>

          <section className="profile-section">
            <div className="profile-section-heading">
              <div><h2>Work history</h2><p>Use one entry per company and keep its achievements together.</p></div>
              <button type="button" className="secondary-action" onClick={() => setProfile((current) => ({ ...current, workHistory: [...current.workHistory, { company: "", title: "", startDate: "", endDate: "", body: "" }] }))}>Add role</button>
            </div>
            {profile.workHistory.length === 0 ? <p className="profile-empty">No work history yet.</p> : (
              <div className="history-editor">
                {profile.workHistory.map((entry, index) => (
                  <div className="history-entry" key={index}>
                    <div className="history-fields">
                      <label>Company<input value={entry.company} onChange={(event) => updateWorkHistory(index, "company", event.target.value)} /></label>
                      <label>Role<input value={entry.title} onChange={(event) => updateWorkHistory(index, "title", event.target.value)} /></label>
                      <label>Start<input value={entry.startDate} onChange={(event) => updateWorkHistory(index, "startDate", event.target.value)} placeholder="2023" /></label>
                      <label>End<input value={entry.endDate} onChange={(event) => updateWorkHistory(index, "endDate", event.target.value)} placeholder="Present" /></label>
                    </div>
                    <label>Experience<textarea value={entry.body} onChange={(event) => updateWorkHistory(index, "body", event.target.value)} rows={7} /></label>
                    <button type="button" className="remove-skill" onClick={() => setProfile((current) => ({ ...current, workHistory: current.workHistory.filter((_, entryIndex) => entryIndex !== index) }))}>Remove role</button>
                  </div>
                ))}
              </div>
            )}
          </section>

          <section className="profile-section">
            <div className="profile-section-heading">
              <div><h2>Education</h2><p>Keep one entry per qualification or institution.</p></div>
              <button type="button" className="secondary-action" onClick={() => setProfile((current) => ({ ...current, education: [...current.education, { institution: "", degree: "", startDate: "", endDate: "", body: "" }] }))}>Add education</button>
            </div>
            {profile.education.length === 0 ? <p className="profile-empty">No education yet.</p> : (
              <div className="history-editor">
                {profile.education.map((entry, index) => (
                  <div className="history-entry" key={index}>
                    <div className="history-fields">
                      <label>Institution<input value={entry.institution} onChange={(event) => updateEducation(index, "institution", event.target.value)} /></label>
                      <label>Qualification<input value={entry.degree} onChange={(event) => updateEducation(index, "degree", event.target.value)} /></label>
                      <label>Start<input value={entry.startDate} onChange={(event) => updateEducation(index, "startDate", event.target.value)} placeholder="2015" /></label>
                      <label>End<input value={entry.endDate} onChange={(event) => updateEducation(index, "endDate", event.target.value)} placeholder="2019" /></label>
                    </div>
                    <label>Details<textarea value={entry.body} onChange={(event) => updateEducation(index, "body", event.target.value)} rows={4} /></label>
                    <button type="button" className="remove-skill" onClick={() => setProfile((current) => ({ ...current, education: current.education.filter((_, entryIndex) => entryIndex !== index) }))}>Remove education</button>
                  </div>
                ))}
              </div>
            )}
          </section>

          <section className="profile-section">
            <div className="profile-section-heading">
              <div><h2>Skills</h2><p>Use a level you can stand behind and add context that makes the claim useful.</p></div>
              <button type="button" className="secondary-action" onClick={() => setProfile((current) => ({ ...current, skills: [...current.skills, { name: "", level: "", notes: "" }] }))}>Add skill</button>
            </div>
            {profile.skills.length === 0 ? <p className="profile-empty">No skills yet. Add the capabilities you want the matcher to assess.</p> : (
              <div className="skill-editor">
                {profile.skills.map((skill, index) => (
                  <div className="skill-row" key={index}>
                    <label>Skill<input value={skill.name} onChange={(event) => updateSkill(index, "name", event.target.value)} placeholder="Go" /></label>
                    <label>Level<select value={skill.level} onChange={(event) => updateSkill(index, "level", event.target.value)}><option value="">Choose</option><option value="learning">Learning</option><option value="working">Working</option><option value="strong">Strong</option><option value="expert">Expert</option></select></label>
                    <label>Notes<input value={skill.notes} onChange={(event) => updateSkill(index, "notes", event.target.value)} placeholder="4 years building and operating APIs" /></label>
                    <button type="button" className="remove-skill" onClick={() => setProfile((current) => ({ ...current, skills: current.skills.filter((_, skillIndex) => skillIndex !== index) }))}>Remove</button>
                  </div>
                ))}
              </div>
            )}
          </section>

          {error && <p className="query-error">{error}</p>}
          <div className="profile-actions">
            <button className="save-profile" disabled={saving}>{saving ? "Saving..." : "Save profile"}</button>
            {(savedAt || profile.updatedAt) && <span>Saved {new Intl.DateTimeFormat(undefined, { dateStyle: "medium", timeStyle: "short" }).format(new Date(savedAt || profile.updatedAt))}</span>}
          </div>
        </form>
      )}
    </section>
  );
}
