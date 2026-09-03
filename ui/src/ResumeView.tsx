import { useEffect, useState, type Dispatch, type FormEvent, type SetStateAction } from "react";
import { fetchResume, saveResume, type Resume } from "./api";

const emptyResume: Resume = {
  id: 1,
  fullName: "",
  headline: "",
  location: "",
  email: "",
  phone: "",
  summary: "",
  links: [],
  skills: [],
  competencies: [],
  experience: [],
  education: [],
  updatedAt: "",
};

export default function ResumeView() {
  const [resume, setResume] = useState<Resume>(emptyResume);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [savedAt, setSavedAt] = useState("");

  useEffect(() => {
    const controller = new AbortController();
    async function load() {
      try {
        const result = await fetchResume(controller.signal);
        if (!controller.signal.aborted) {
          setResume(result.resume ?? emptyResume);
          setError("");
        }
      } catch (reason) {
        if (!(reason instanceof DOMException && reason.name === "AbortError")) {
          setError(reason instanceof Error ? reason.message : "Could not load resume");
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

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSaving(true);
    try {
      const result = await saveResume(resume);
      setResume(result.resume);
      setSavedAt(result.resume.updatedAt);
      setError("");
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "Could not save resume");
    } finally {
      setSaving(false);
    }
  }

  function updateHeader(field: "fullName" | "headline" | "location" | "email" | "phone" | "summary", value: string) {
    setResume((current) => ({ ...current, [field]: value }));
  }

  return (
    <section className="profile-page">
      <header className="profile-header">
        <div>
          <p className="eyebrow">Base resume</p>
          <h1>Your application-ready resume</h1>
          <p>Keep this concise and factual. It is separate from the profile used for job matching.</p>
        </div>
      </header>

      {loading ? <p className="browse-loading">Loading resume...</p> : (
        <form className="profile-form" onSubmit={(event) => void submit(event)}>
          <section className="profile-section">
            <h2>Header</h2>
            <div className="profile-fields">
              <label>Full name<input value={resume.fullName} onChange={(event) => updateHeader("fullName", event.target.value)} /></label>
              <label>Professional headline<input value={resume.headline} onChange={(event) => updateHeader("headline", event.target.value)} placeholder="Senior backend engineer" /></label>
              <label>Location<input value={resume.location} onChange={(event) => updateHeader("location", event.target.value)} placeholder="Berlin, Germany" /></label>
              <label>Email<input type="email" value={resume.email} onChange={(event) => updateHeader("email", event.target.value)} /></label>
              <label>Phone<input type="tel" value={resume.phone} onChange={(event) => updateHeader("phone", event.target.value)} /></label>
            </div>
            <label className="profile-summary">Professional summary<textarea value={resume.summary} onChange={(event) => updateHeader("summary", event.target.value)} rows={6} placeholder="A concise overview of your experience, strengths, and target role." /></label>
          </section>

          <LinksSection resume={resume} setResume={setResume} />
          <SkillsSection resume={resume} setResume={setResume} />
          <CompetenciesSection resume={resume} setResume={setResume} />
          <ExperienceSection resume={resume} setResume={setResume} />
          <EducationSection resume={resume} setResume={setResume} />

          {error && <p className="query-error">{error}</p>}
          <div className="profile-actions">
            <button className="save-profile" disabled={saving}>{saving ? "Saving..." : "Save base resume"}</button>
            {(savedAt || resume.updatedAt) && <span>Saved {new Intl.DateTimeFormat(undefined, { dateStyle: "medium", timeStyle: "short" }).format(new Date(savedAt || resume.updatedAt))}</span>}
          </div>
        </form>
      )}
    </section>
  );
}

function LinksSection({ resume, setResume }: ResumeSectionProps) {
  return <section className="profile-section">
    <SectionHeading title="Links" description="Include public professional links that belong on the PDF." action="Add link" onAdd={() => setResume((current) => ({ ...current, links: [...current.links, { label: "", url: "" }] }))} />
    {resume.links.length === 0 ? <Empty message="No links yet." /> : <div className="resume-link-editor">{resume.links.map((link, index) => <div className="resume-link-row" key={index}>
      <label>Label<input value={link.label} onChange={(event) => setResume((current) => ({ ...current, links: current.links.map((item, itemIndex) => itemIndex === index ? { ...item, label: event.target.value } : item) }))} placeholder="LinkedIn" /></label>
      <label>URL<input type="url" value={link.url} onChange={(event) => setResume((current) => ({ ...current, links: current.links.map((item, itemIndex) => itemIndex === index ? { ...item, url: event.target.value } : item) }))} placeholder="https://..." /></label>
      <RemoveButton label="Remove" onClick={() => setResume((current) => ({ ...current, links: current.links.filter((_, itemIndex) => itemIndex !== index) }))} />
    </div>)}</div>}
  </section>;
}

function SkillsSection({ resume, setResume }: ResumeSectionProps) {
  return <section className="profile-section">
    <SectionHeading title="Key competences" description="These render as a compact, ATS-readable skills line." action="Add competence" onAdd={() => setResume((current) => ({ ...current, skills: [...current.skills, { name: "" }] }))} />
    {resume.skills.length === 0 ? <Empty message="No skills yet." /> : <div className="resume-skill-editor">{resume.skills.map((skill, index) => <div className="resume-skill-row" key={index}>
      <label>Skill<input value={skill.name} onChange={(event) => setResume((current) => ({ ...current, skills: current.skills.map((item, itemIndex) => itemIndex === index ? { ...item, name: event.target.value } : item) }))} placeholder="Go" /></label>
      <RemoveButton label="Remove" onClick={() => setResume((current) => ({ ...current, skills: current.skills.filter((_, itemIndex) => itemIndex !== index) }))} />
    </div>)}</div>}
  </section>;
}

function CompetenciesSection({ resume, setResume }: ResumeSectionProps) {
  return <section className="profile-section">
    <SectionHeading title="Key competencies" description="Group high-value evidence into short, focused sections." action="Add competency" onAdd={() => setResume((current) => ({ ...current, competencies: [...current.competencies, { title: "", bullets: [] }] }))} />
    {resume.competencies.length === 0 ? <Empty message="No competency sections yet." /> : <div className="history-editor">{resume.competencies.map((competency, index) => <div className="history-entry" key={index}>
      <label>Heading<input value={competency.title} onChange={(event) => setResume((current) => ({ ...current, competencies: current.competencies.map((item, itemIndex) => itemIndex === index ? { ...item, title: event.target.value } : item) }))} placeholder="Backend systems" /></label>
      <BulletEditor bullets={competency.bullets} onChange={(bulletIndex, value) => setResume((current) => ({ ...current, competencies: current.competencies.map((item, itemIndex) => itemIndex === index ? { ...item, bullets: item.bullets.map((bullet, index) => index === bulletIndex ? value : bullet) } : item) }))} onAdd={() => setResume((current) => ({ ...current, competencies: current.competencies.map((item, itemIndex) => itemIndex === index ? { ...item, bullets: [...item.bullets, ""] } : item) }))} onRemove={(bulletIndex) => setResume((current) => ({ ...current, competencies: current.competencies.map((item, itemIndex) => itemIndex === index ? { ...item, bullets: item.bullets.filter((_, index) => index !== bulletIndex) } : item) }))} />
      <RemoveButton label="Remove competency" onClick={() => setResume((current) => ({ ...current, competencies: current.competencies.filter((_, itemIndex) => itemIndex !== index) }))} />
    </div>)}</div>}
  </section>;
}

function ExperienceSection({ resume, setResume }: ResumeSectionProps) {
  return <section className="profile-section">
    <SectionHeading title="Work experience" description="Use impact-focused bullets. Dates should use YYYY-MM." action="Add role" onAdd={() => setResume((current) => ({ ...current, experience: [...current.experience, { company: "", title: "", location: "", startDate: "", endDate: "", isCurrent: false, stack: "", bullets: [] }] }))} />
    {resume.experience.length === 0 ? <Empty message="No work experience yet." /> : <div className="history-editor">{resume.experience.map((entry, index) => <div className="history-entry" key={index}>
      <div className="history-fields"><label>Company<input value={entry.company} onChange={(event) => updateExperience(setResume, index, "company", event.target.value)} /></label><label>Role<input value={entry.title} onChange={(event) => updateExperience(setResume, index, "title", event.target.value)} /></label><label>Start<input value={entry.startDate} onChange={(event) => updateExperience(setResume, index, "startDate", event.target.value)} placeholder="2023-01" /></label><label>End<input value={entry.endDate} disabled={entry.isCurrent} onChange={(event) => updateExperience(setResume, index, "endDate", event.target.value)} placeholder="2025-06" /></label></div>
      <div className="profile-fields"><label>Location<input value={entry.location} onChange={(event) => updateExperience(setResume, index, "location", event.target.value)} /></label><label>Stack<input value={entry.stack} onChange={(event) => updateExperience(setResume, index, "stack", event.target.value)} placeholder="Go, PostgreSQL, Kubernetes" /></label></div>
      <label className="resume-current"><input type="checkbox" checked={entry.isCurrent} onChange={(event) => setResume((current) => ({ ...current, experience: current.experience.map((item, itemIndex) => itemIndex === index ? { ...item, isCurrent: event.target.checked, endDate: event.target.checked ? "" : item.endDate } : item) }))} />Current role</label>
      <BulletEditor bullets={entry.bullets} onChange={(bulletIndex, value) => setResume((current) => ({ ...current, experience: current.experience.map((item, itemIndex) => itemIndex === index ? { ...item, bullets: item.bullets.map((bullet, index) => index === bulletIndex ? value : bullet) } : item) }))} onAdd={() => setResume((current) => ({ ...current, experience: current.experience.map((item, itemIndex) => itemIndex === index ? { ...item, bullets: [...item.bullets, ""] } : item) }))} onRemove={(bulletIndex) => setResume((current) => ({ ...current, experience: current.experience.map((item, itemIndex) => itemIndex === index ? { ...item, bullets: item.bullets.filter((_, index) => index !== bulletIndex) } : item) }))} />
      <RemoveButton label="Remove role" onClick={() => setResume((current) => ({ ...current, experience: current.experience.filter((_, itemIndex) => itemIndex !== index) }))} />
    </div>)}</div>}
  </section>;
}

function EducationSection({ resume, setResume }: ResumeSectionProps) {
  return <section className="profile-section">
    <SectionHeading title="Education" description="Include qualifications relevant to the roles you apply for." action="Add education" onAdd={() => setResume((current) => ({ ...current, education: [...current.education, { institution: "", location: "", degree: "", fieldOfStudy: "", startDate: "", endDate: "", details: "" }] }))} />
    {resume.education.length === 0 ? <Empty message="No education yet." /> : <div className="history-editor">{resume.education.map((entry, index) => <div className="history-entry" key={index}>
      <div className="profile-fields"><label>Institution<input value={entry.institution} onChange={(event) => updateEducation(setResume, index, "institution", event.target.value)} /></label><label>Location<input value={entry.location} onChange={(event) => updateEducation(setResume, index, "location", event.target.value)} /></label><label>Qualification<input value={entry.degree} onChange={(event) => updateEducation(setResume, index, "degree", event.target.value)} /></label><label>Field of study<input value={entry.fieldOfStudy} onChange={(event) => updateEducation(setResume, index, "fieldOfStudy", event.target.value)} /></label><label>Start<input value={entry.startDate} onChange={(event) => updateEducation(setResume, index, "startDate", event.target.value)} placeholder="2019" /></label><label>End<input value={entry.endDate} onChange={(event) => updateEducation(setResume, index, "endDate", event.target.value)} placeholder="2023" /></label></div>
      <label>Details<textarea value={entry.details} onChange={(event) => updateEducation(setResume, index, "details", event.target.value)} rows={3} /></label>
      <RemoveButton label="Remove education" onClick={() => setResume((current) => ({ ...current, education: current.education.filter((_, itemIndex) => itemIndex !== index) }))} />
    </div>)}</div>}
  </section>;
}

type ResumeSectionProps = { resume: Resume; setResume: Dispatch<SetStateAction<Resume>> };

function updateExperience(setResume: ResumeSectionProps["setResume"], index: number, field: "company" | "title" | "location" | "startDate" | "endDate" | "stack", value: string) {
  setResume((current) => ({ ...current, experience: current.experience.map((entry, entryIndex) => entryIndex === index ? { ...entry, [field]: value } : entry) }));
}

function updateEducation(setResume: ResumeSectionProps["setResume"], index: number, field: "institution" | "location" | "degree" | "fieldOfStudy" | "startDate" | "endDate" | "details", value: string) {
  setResume((current) => ({ ...current, education: current.education.map((entry, entryIndex) => entryIndex === index ? { ...entry, [field]: value } : entry) }));
}

function SectionHeading({ title, description, action, onAdd }: { title: string; description: string; action: string; onAdd: () => void }) {
  return <div className="profile-section-heading"><div><h2>{title}</h2><p>{description}</p></div><button type="button" className="secondary-action" onClick={onAdd}>{action}</button></div>;
}

function Empty({ message }: { message: string }) {
  return <p className="profile-empty">{message}</p>;
}

function RemoveButton({ label, onClick }: { label: string; onClick: () => void }) {
  return <button type="button" className="remove-skill" onClick={onClick}>{label}</button>;
}

function BulletEditor({ bullets, onAdd, onChange, onRemove }: { bullets: string[]; onAdd: () => void; onChange: (index: number, value: string) => void; onRemove: (index: number) => void }) {
  return <div className="resume-bullets"><div className="resume-bullet-heading"><strong>Bullets</strong><button type="button" className="secondary-action" onClick={onAdd}>Add bullet</button></div>{bullets.map((bullet, index) => <div className="resume-bullet-row" key={index}><textarea value={bullet} onChange={(event) => onChange(index, event.target.value)} rows={2} placeholder="Describe an outcome, responsibility, or achievement." /><RemoveButton label="Remove" onClick={() => onRemove(index)} /></div>)}</div>;
}
