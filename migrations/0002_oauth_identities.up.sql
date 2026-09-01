-- 0002_oauth_identities.up.sql

CREATE TABLE oauth_identities (
    id          UUID PRIMARY KEY,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider    TEXT NOT NULL,
    external_id TEXT NOT NULL,
    email       TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Backstops OAuthStore.GetByProviderID and is the real guard
    -- against ever double-linking the same external account.
    UNIQUE (provider, external_id)
);

CREATE INDEX idx_oauth_identities_user_id ON oauth_identities(user_id);
