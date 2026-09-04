-- +goose Up
DROP TABLE IF EXISTS application_resume_agent_events;
DROP TABLE IF EXISTS application_resume_revisions;
DROP TABLE IF EXISTS application_resumes;
DROP TABLE IF EXISTS job_match_chat_messages;

CREATE TABLE job_match_chat_items (
    id INTEGER PRIMARY KEY,
    job_id INTEGER NOT NULL REFERENCES job_matches(job_id) ON DELETE CASCADE,
    sequence INTEGER NOT NULL,
    item_type TEXT NOT NULL,
    payload_json TEXT NOT NULL,
    request_id TEXT,
    created_at TEXT NOT NULL,
    UNIQUE(job_id, sequence)
);

CREATE INDEX job_match_chat_items_job_id_sequence ON job_match_chat_items(job_id, sequence);
CREATE UNIQUE INDEX job_match_chat_items_job_id_request_id_type
    ON job_match_chat_items(job_id, request_id, item_type)
    WHERE request_id IS NOT NULL AND item_type = 'user_message';

-- +goose Down
DROP INDEX job_match_chat_items_job_id_request_id_type;
DROP INDEX job_match_chat_items_job_id_sequence;
DROP TABLE job_match_chat_items;

CREATE TABLE job_match_chat_messages (
    id INTEGER PRIMARY KEY,
    job_id INTEGER NOT NULL REFERENCES job_matches(job_id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK (role IN ('user', 'assistant')),
    content TEXT NOT NULL,
    created_at TEXT NOT NULL,
    request_id TEXT
);
CREATE INDEX job_match_chat_messages_job_id_id ON job_match_chat_messages(job_id, id);
CREATE UNIQUE INDEX job_match_chat_messages_job_id_request_id_role
    ON job_match_chat_messages(job_id, request_id, role)
    WHERE request_id IS NOT NULL;

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
