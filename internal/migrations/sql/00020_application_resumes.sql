-- +goose Up
CREATE TABLE application_resumes (
    id INTEGER PRIMARY KEY,
    job_id INTEGER NOT NULL UNIQUE REFERENCES job_matches(job_id) ON DELETE CASCADE,
    root_message_id INTEGER NOT NULL UNIQUE REFERENCES job_match_chat_messages(id) ON DELETE CASCADE,
    base_resume_json TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE TABLE application_resume_revisions (
    id INTEGER PRIMARY KEY,
    application_resume_id INTEGER NOT NULL REFERENCES application_resumes(id) ON DELETE CASCADE,
    revision_number INTEGER NOT NULL,
    trigger_message_id INTEGER NOT NULL REFERENCES job_match_chat_messages(id) ON DELETE CASCADE,
    assistant_message_id INTEGER REFERENCES job_match_chat_messages(id) ON DELETE CASCADE,
    resume_json TEXT NOT NULL,
    summary TEXT NOT NULL,
    created_at TEXT NOT NULL,
    UNIQUE(application_resume_id, revision_number)
);

CREATE TABLE application_resume_agent_events (
    id INTEGER PRIMARY KEY,
    trigger_message_id INTEGER NOT NULL REFERENCES job_match_chat_messages(id) ON DELETE CASCADE,
    revision_id INTEGER REFERENCES application_resume_revisions(id) ON DELETE SET NULL,
    event_type TEXT NOT NULL,
    detail TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE INDEX application_resume_revisions_application_id_number ON application_resume_revisions(application_resume_id, revision_number);
CREATE INDEX application_resume_agent_events_trigger_message_id ON application_resume_agent_events(trigger_message_id, id);

-- +goose Down
DROP INDEX application_resume_agent_events_trigger_message_id;
DROP INDEX application_resume_revisions_application_id_number;
DROP TABLE application_resume_agent_events;
DROP TABLE application_resume_revisions;
DROP TABLE application_resumes;
