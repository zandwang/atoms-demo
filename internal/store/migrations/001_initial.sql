CREATE TABLE workspaces (
    id TEXT PRIMARY KEY,
    display_name TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE sessions (
    token_hash BLOB PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE INDEX idx_sessions_workspace_id ON sessions(workspace_id);
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);

CREATE TABLE projects (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    summary TEXT NOT NULL DEFAULT '',
    active_version_id TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX idx_projects_workspace_updated_at ON projects(workspace_id, updated_at DESC);

CREATE TABLE messages (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    role TEXT NOT NULL,
    content TEXT NOT NULL,
    generation_id TEXT,
    created_at TEXT NOT NULL
);

CREATE INDEX idx_messages_project_created_at ON messages(project_id, created_at);

CREATE TABLE generation_attempts (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    status TEXT NOT NULL,
    error_code TEXT,
    error_message TEXT,
    started_at TEXT NOT NULL,
    completed_at TEXT
);

CREATE INDEX idx_generation_attempts_project_started_at ON generation_attempts(project_id, started_at DESC);

CREATE TABLE versions (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    parent_version_id TEXT,
    sequence INTEGER NOT NULL,
    request TEXT NOT NULL,
    plan_json TEXT NOT NULL,
    spec_json TEXT NOT NULL,
    artifact_entry_html TEXT NOT NULL,
    artifact_html TEXT NOT NULL,
    artifact_css TEXT NOT NULL,
    artifact_js TEXT NOT NULL,
    artifact_manifest_json TEXT NOT NULL,
    checksum TEXT NOT NULL,
    created_at TEXT NOT NULL,
    UNIQUE(project_id, sequence)
);

CREATE INDEX idx_versions_project_created_at ON versions(project_id, created_at DESC);

CREATE TABLE preview_states (
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    version_id TEXT NOT NULL REFERENCES versions(id) ON DELETE CASCADE,
    state_json TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    PRIMARY KEY(project_id, version_id)
);
