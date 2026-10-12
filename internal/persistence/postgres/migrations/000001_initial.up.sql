BEGIN;

CREATE TABLE users (
    id uuid PRIMARY KEY,
    username text NOT NULL UNIQUE CHECK (btrim(username) <> ''),
    first_name text NOT NULL,
    last_name text NOT NULL,
    password_hash text NOT NULL,
    role text NOT NULL CHECK (role IN ('admin', 'user')),
    needs_password_reset boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE projects (
    id uuid PRIMARY KEY,
    name text NOT NULL UNIQUE CHECK (btrim(name) <> ''),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE clients (
    id uuid PRIMARY KEY,
    project_id uuid NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
    name text NOT NULL CHECK (btrim(name) <> ''),
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (project_id, name)
);

CREATE TABLE user_projects (
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, project_id)
);
CREATE INDEX user_projects_project_idx ON user_projects(project_id);

CREATE TABLE login_tokens (
    token_hash bytea PRIMARY KEY CHECK (octet_length(token_hash) = 32),
    user_id uuid NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    CHECK (expires_at > created_at)
);

CREATE TABLE sessions (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    CHECK (expires_at > created_at)
);
CREATE INDEX sessions_user_idx ON sessions(user_id);
CREATE INDEX sessions_expiry_idx ON sessions(expires_at);

CREATE TABLE archive_policies (
    client_id uuid PRIMARY KEY REFERENCES clients(id) ON DELETE RESTRICT,
    archive_after_weeks integer NOT NULL CHECK (archive_after_weeks > 0),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE archives (
    id uuid PRIMARY KEY,
    client_id uuid NOT NULL REFERENCES clients(id) ON DELETE RESTRICT,
    status text NOT NULL DEFAULT 'building'
        CHECK (status IN ('building', 'ready', 'deleting')),
    storage_backend text NOT NULL CHECK (storage_backend IN ('disk', 's3')),
    object_key text NOT NULL,
    file_name text NOT NULL,
    size_bytes bigint CHECK (size_bytes >= 0),
    sha256 bytea CHECK (octet_length(sha256) = 32),
    log_count bigint NOT NULL DEFAULT 0 CHECK (log_count >= 0),
    cutoff_at timestamptz NOT NULL,
    claimed_at timestamptz,
    from_at timestamptz,
    to_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (storage_backend, object_key),
    UNIQUE (client_id, id),
    CHECK (from_at IS NULL OR to_at IS NULL OR from_at <= to_at),
    CHECK (status <> 'ready' OR (
        size_bytes IS NOT NULL AND sha256 IS NOT NULL AND
        from_at IS NOT NULL AND to_at IS NOT NULL AND log_count > 0
    ))
);
CREATE INDEX archives_client_created_idx ON archives(client_id, created_at DESC);
CREATE UNIQUE INDEX archives_one_building_per_client_idx
    ON archives(client_id) WHERE status = 'building';

CREATE TABLE logs (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    client_id uuid NOT NULL REFERENCES clients(id) ON DELETE RESTRICT,
    instance_name text NOT NULL,
    occurred_at timestamptz NOT NULL,
    received_at timestamptz NOT NULL DEFAULT now(),
    sequence_number bigint NOT NULL CHECK (sequence_number >= 0),
    message text NOT NULL,
    archive_id uuid,
    FOREIGN KEY (client_id, archive_id)
        REFERENCES archives(client_id, id) ON DELETE RESTRICT
);
CREATE INDEX logs_client_order_idx
    ON logs(client_id, occurred_at DESC, sequence_number DESC, id DESC);
CREATE INDEX logs_client_instance_order_idx
    ON logs(client_id, instance_name, occurred_at DESC, sequence_number DESC, id DESC);
CREATE INDEX logs_archive_idx
    ON logs(archive_id) WHERE archive_id IS NOT NULL;
CREATE INDEX logs_search_idx
    ON logs USING gin(to_tsvector('simple'::regconfig, message));

COMMIT;
