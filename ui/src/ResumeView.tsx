import { useEffect, useState, type ChangeEvent, type Dispatch, type FormEvent, type SetStateAction } from "react";
import { fetchResume, resumePDFURL, resumePhotoURL, saveResume, uploadResumePhoto, type Resume } from "./api";

const emptyResume: Resume = {
  id: 1,
  fullName: "",
  headline: "",
  town: "",
  country: "",
  email: "",
  phone: "",
  summaryParagraphs: [],
  links: [],
  skills: [],
  experience: [],
  education: [],
  hasPhoto: false,
  updatedAt: "",
};

export default function ResumeView() {
  const [resume, setResume] = useState<Resume>(emptyResume);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [uploadingPhoto, setUploadingPhoto] = useState(false);
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

  function updateHeader(field: "fullName" | "headline" | "town" | "country" | "email" | "phone", value: string) {
    setResume((current) => ({ ...current, [field]: value }));
  }

  async function uploadPhoto(event: ChangeEvent<HTMLInputElement>) {
    const photo = event.target.files?.[0];
    event.target.value = "";
    if (!photo) {
      return;
    }
    setUploadingPhoto(true);
    try {
      const result = await uploadResumePhoto(photo);
      setResume((current) => ({ ...current, hasPhoto: result.resume.hasPhoto, updatedAt: result.resume.updatedAt }));
      setError("");
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "Could not upload resume photo");
    } finally {
      setUploadingPhoto(false);
    }
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
        <form className="profile-form" noValidate onSubmit={(event) => void submit(event)}>
          <section className="profile-section">
            <h2>Header</h2>
            <div className="profile-fields">
              <label>Full name<input value={resume.fullName} onChange={(event) => updateHeader("fullName", event.target.value)} /></label>
              <label>Professional headline<input value={resume.headline} onChange={(event) => updateHeader("headline", event.target.value)} placeholder="Senior backend engineer" /></label>
              <label>Town<input value={resume.town} onChange={(event) => updateHeader("town", event.target.value)} placeholder="Berlin" /></label>
              <label>Country<input value={resume.country} onChange={(event) => updateHeader("country", event.target.value)} placeholder="Germany" /></label>
              <label>Email<input type="email" value={resume.email} onChange={(event) => updateHeader("email", event.target.value)} /></label>
              <label>Phone<input type="tel" value={resume.phone} onChange={(event) => updateHeader("phone", event.target.value)} /></label>
            </div>
            <div className="resume-photo-upload">
              <div><strong>Resume photo</strong><p>Optional JPEG or PNG, up to 5 MB. Save the resume before uploading.</p></div>
              {resume.hasPhoto && <img src={`${resumePhotoURL()}?updatedAt=${encodeURIComponent(resume.updatedAt)}`} alt="Resume" />}
              <label className="secondary-action">{uploadingPhoto ? "Uploading..." : "Upload photo"}<input type="file" accept="image/jpeg,image/png" disabled={uploadingPhoto} onChange={(event) => void uploadPhoto(event)} /></label>
            </div>
          </section>

          <SummarySection resume={resume} setResume={setResume} />
          <LinksSection resume={resume} setResume={setResume} />
          <ExperienceSection resume={resume} setResume={setResume} />
          <EducationSection resume={resume} setResume={setResume} />
          <SkillsSection resume={resume} setResume={setResume} />

          {error && <p className="query-error">{error}</p>}
           <div className="profile-actions">
             <button className="save-profile" disabled={saving}>{saving ? "Saving..." : "Save base resume"}</button>
             {resume.updatedAt && <a className="download-resume" href={resumePDFURL()}>Download PDF</a>}
            {(savedAt || resume.updatedAt) && <span>Saved {new Intl.DateTimeFormat(undefined, { dateStyle: "medium", timeStyle: "short" }).format(new Date(savedAt || resume.updatedAt))}</span>}
          </div>
        </form>
      )}
    </section>
  );
}

function SummarySection({ resume, setResume }: ResumeSectionProps) {
  return <section className="profile-section">
    <SectionHeading title="Professional summary" description="Each field renders as a separate paragraph in the PDF." action="Add paragraph" onAdd={() => setResume((current) => ({ ...current, summaryParagraphs: [...current.summaryParagraphs, { id: 0, content: "" }] }))} />
    {resume.summaryParagraphs.length === 0 ? <Empty message="No summary paragraphs yet." /> : <div className="resume-summary-editor">{resume.summaryParagraphs.map((paragraph, index) => <div className="resume-summary-row" key={paragraph.id || `new-${index}`}>
      <label>Paragraph {index + 1}<textarea aria-label={`Professional summary paragraph ${index + 1}`} value={paragraph.content} onChange={(event) => setResume((current) => ({ ...current, summaryParagraphs: current.summaryParagraphs.map((item, itemIndex) => itemIndex === index ? { ...item, content: event.target.value } : item) }))} rows={4} placeholder="Describe your experience, strengths, or target role." /></label>
      <RemoveButton label="Remove paragraph" onClick={() => setResume((current) => ({ ...current, summaryParagraphs: current.summaryParagraphs.filter((_, itemIndex) => itemIndex !== index) }))} />
    </div>)}</div>}
  </section>;
}

function LinksSection({ resume, setResume }: ResumeSectionProps) {
  return <section className="profile-section">
    <SectionHeading title="Links" description="Include public professional links that belong on the PDF." action="Add link" onAdd={() => setResume((current) => ({ ...current, links: [...current.links, { id: 0, label: "", url: "" }] }))} />
    {resume.links.length === 0 ? <Empty message="No links yet." /> : <div className="resume-link-editor">{resume.links.map((link, index) => <div className="resume-link-row" key={link.id || `new-${index}`}>
      <label>Label<input value={link.label} onChange={(event) => setResume((current) => ({ ...current, links: current.links.map((item, itemIndex) => itemIndex === index ? { ...item, label: event.target.value } : item) }))} placeholder="LinkedIn" /></label>
      <label>URL<input type="url" value={link.url} onChange={(event) => setResume((current) => ({ ...current, links: current.links.map((item, itemIndex) => itemIndex === index ? { ...item, url: event.target.value } : item) }))} placeholder="https://..." /></label>
      <RemoveButton label="Remove" onClick={() => setResume((current) => ({ ...current, links: current.links.filter((_, itemIndex) => itemIndex !== index) }))} />
    </div>)}</div>}
  </section>;
}

function SkillsSection({ resume, setResume }: ResumeSectionProps) {
  return <section className="profile-section">
    <SectionHeading title="Skills" description="These render as compact, ATS-readable tags at the end of the PDF." action="Add skill" onAdd={() => setResume((current) => ({ ...current, skills: [...current.skills, { id: 0, name: "" }] }))} />
    {resume.skills.length === 0 ? <Empty message="No skills yet." /> : <div className="resume-skill-editor">{resume.skills.map((skill, index) => <div className="resume-skill-row" key={skill.id || `new-${index}`}>
      <label>Skill<input value={skill.name} onChange={(event) => setResume((current) => ({ ...current, skills: current.skills.map((item, itemIndex) => itemIndex === index ? { ...item, name: event.target.value } : item) }))} placeholder="Go" /></label>
      <RemoveButton label="Remove" onClick={() => setResume((current) => ({ ...current, skills: current.skills.filter((_, itemIndex) => itemIndex !== index) }))} />
    </div>)}</div>}
  </section>;
}

function ExperienceSection({ resume, setResume }: ResumeSectionProps) {
  return <section className="profile-section">
    <SectionHeading title="Work experience" description="Use impact-focused bullets. Dates should use YYYY-MM." action="Add role" onAdd={() => setResume((current) => ({ ...current, experience: [...current.experience, { id: 0, company: "", title: "", location: "", startDate: "", endDate: "", isCurrent: false, stack: "", bullets: [] }] }))} />
    {resume.experience.length === 0 ? <Empty message="No work experience yet." /> : <div className="history-editor">{resume.experience.map((entry, index) => <div className="history-entry" key={entry.id || `new-${index}`}>
      <div className="history-fields"><label>Company<input value={entry.company} onChange={(event) => updateExperience(setResume, index, "company", event.target.value)} /></label><label>Role<input value={entry.title} onChange={(event) => updateExperience(setResume, index, "title", event.target.value)} /></label><label>Start<input value={entry.startDate} onChange={(event) => updateExperience(setResume, index, "startDate", event.target.value)} placeholder="2023-01" /></label><label>End<input value={entry.endDate} disabled={entry.isCurrent} onChange={(event) => updateExperience(setResume, index, "endDate", event.target.value)} placeholder="2025-06" /></label></div>
      <div className="profile-fields"><label>Location<input value={entry.location} onChange={(event) => updateExperience(setResume, index, "location", event.target.value)} /></label><label>Stack<input value={entry.stack} onChange={(event) => updateExperience(setResume, index, "stack", event.target.value)} placeholder="Go, PostgreSQL, Kubernetes" /></label></div>
      <label className="resume-current"><input type="checkbox" checked={entry.isCurrent} onChange={(event) => setResume((current) => ({ ...current, experience: current.experience.map((item, itemIndex) => itemIndex === index ? { ...item, isCurrent: event.target.checked, endDate: event.target.checked ? "" : item.endDate } : item) }))} />Current role</label>
      <BulletEditor bullets={entry.bullets} onChange={(bulletIndex, value) => setResume((current) => ({ ...current, experience: current.experience.map((item, itemIndex) => itemIndex === index ? { ...item, bullets: item.bullets.map((bullet, index) => index === bulletIndex ? { ...bullet, content: value } : bullet) } : item) }))} onAdd={() => setResume((current) => ({ ...current, experience: current.experience.map((item, itemIndex) => itemIndex === index ? { ...item, bullets: [...item.bullets, { id: 0, content: "" }] } : item) }))} onRemove={(bulletIndex) => setResume((current) => ({ ...current, experience: current.experience.map((item, itemIndex) => itemIndex === index ? { ...item, bullets: item.bullets.filter((_, index) => index !== bulletIndex) } : item) }))} />
      <RemoveButton label="Remove role" onClick={() => setResume((current) => ({ ...current, experience: current.experience.filter((_, itemIndex) => itemIndex !== index) }))} />
    </div>)}</div>}
  </section>;
}

function EducationSection({ resume, setResume }: ResumeSectionProps) {
  return <section className="profile-section">
    <SectionHeading title="Education" description="Include qualifications relevant to the roles you apply for." action="Add education" onAdd={() => setResume((current) => ({ ...current, education: [...current.education, { id: 0, institution: "", location: "", degree: "", fieldOfStudy: "", startDate: "", endDate: "", details: "" }] }))} />
    {resume.education.length === 0 ? <Empty message="No education yet." /> : <div className="history-editor">{resume.education.map((entry, index) => <div className="history-entry" key={entry.id || `new-${index}`}>
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

function BulletEditor({ bullets, onAdd, onChange, onRemove }: { bullets: { id: number; content: string }[]; onAdd: () => void; onChange: (index: number, value: string) => void; onRemove: (index: number) => void }) {
  return <div className="resume-bullets"><div className="resume-bullet-heading"><strong>Bullets</strong><button type="button" className="secondary-action" onClick={onAdd}>Add bullet</button></div>{bullets.map((bullet, index) => <div className="resume-bullet-row" key={bullet.id || `new-${index}`}><textarea value={bullet.content} onChange={(event) => onChange(index, event.target.value)} rows={2} placeholder="Describe an outcome, responsibility, or achievement." /><RemoveButton label="Remove" onClick={() => onRemove(index)} /></div>)}</div>;
}
