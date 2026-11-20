-- +goose Up
-- One row per approved OAuth authorization of an MCP client. Approval stores a
-- single-use authorization code; exchanging it issues the access and refresh
-- tokens. Only SHA-256 hashes of codes and tokens are stored. Each refresh
-- rotates both tokens. Refresh tokens contain the grant ID, so presenting any
-- earlier refresh token of a grant revokes it. Deleting the row revokes all
-- its tokens.
CREATE TABLE oauth_grants (
    id                 uuid PRIMARY KEY,
    user_id            uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    client_id          text NOT NULL,
    redirect_uri       text NOT NULL,
    resource           text NOT NULL,
    scope              text NOT NULL,
    code_hash          bytea NOT NULL UNIQUE CHECK (octet_length(code_hash) = 32),
    code_challenge     text NOT NULL,
    code_expires_at    timestamptz NOT NULL,
    access_token_hash  bytea UNIQUE CHECK (octet_length(access_token_hash) = 32),
    access_expires_at  timestamptz,
    refresh_token_hash bytea CHECK (octet_length(refresh_token_hash) = 32),
    -- Until the code is exchanged this equals code_expires_at; afterwards it
    -- is the refresh token's expiry.
    expires_at         timestamptz NOT NULL,
    created_at         timestamptz NOT NULL DEFAULT now(),
    CHECK ((access_token_hash IS NULL) = (refresh_token_hash IS NULL)),
    CHECK ((access_token_hash IS NULL) = (access_expires_at IS NULL))
);
CREATE INDEX oauth_grants_user_idx ON oauth_grants(user_id);
CREATE INDEX oauth_grants_expiry_idx ON oauth_grants(expires_at);

-- +goose Down
DROP TABLE oauth_grants;
