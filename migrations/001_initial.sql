CREATE TABLE users (
 id text PRIMARY KEY, email text NOT NULL UNIQUE, password_hash text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE sessions (
 token_hash text PRIMARY KEY, user_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 expires_at timestamptz NOT NULL
);
CREATE INDEX sessions_expiry ON sessions(expires_at);
CREATE TABLE projects (
 id text PRIMARY KEY, owner_id text NOT NULL REFERENCES users(id), name text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX projects_owner ON projects(owner_id, created_at DESC, id);
CREATE TABLE api_keys (
 id text PRIMARY KEY, project_id text NOT NULL REFERENCES projects(id), name text NOT NULL,
 key_hash text NOT NULL UNIQUE, prefix text NOT NULL, revoked_at timestamptz,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX keys_project ON api_keys(project_id,created_at DESC,id);
CREATE TABLE endpoints (
 id text PRIMARY KEY, project_id text NOT NULL REFERENCES projects(id), name text NOT NULL,
 url text NOT NULL, secret_cipher text NOT NULL, active boolean NOT NULL DEFAULT true,
 max_attempts integer NOT NULL DEFAULT 5 CHECK(max_attempts BETWEEN 1 AND 10),
 base_delay_seconds integer NOT NULL DEFAULT 5 CHECK(base_delay_seconds BETWEEN 1 AND 3600),
 rate_per_second integer NOT NULL DEFAULT 10 CHECK(rate_per_second BETWEEN 1 AND 1000),
 created_at timestamptz NOT NULL DEFAULT now(), deleted_at timestamptz
);
CREATE INDEX endpoints_project ON endpoints(project_id) WHERE deleted_at IS NULL;
CREATE TABLE events (
 id text PRIMARY KEY, project_id text NOT NULL REFERENCES projects(id), event_id text NOT NULL,
 type text NOT NULL, payload jsonb NOT NULL, payload_hash text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(), UNIQUE(project_id,event_id)
);
CREATE INDEX events_project_time ON events(project_id,created_at DESC,id);
CREATE INDEX events_type_time ON events(project_id,type,created_at DESC,id);
CREATE TABLE deliveries (
 id text PRIMARY KEY, event_id text NOT NULL REFERENCES events(id), endpoint_id text NOT NULL REFERENCES endpoints(id),
 status text NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','processing','retrying','succeeded','dead','cancelled')),
 attempt_count integer NOT NULL DEFAULT 0, generation integer NOT NULL DEFAULT 0,
 next_attempt_at timestamptz NOT NULL DEFAULT now(), lease_until timestamptz, lease_token text,
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(event_id,endpoint_id)
);
CREATE INDEX deliveries_due ON deliveries(next_attempt_at,id) WHERE status IN ('pending','retrying');
CREATE INDEX deliveries_expired ON deliveries(lease_until) WHERE status='processing';
CREATE INDEX deliveries_event ON deliveries(event_id);
CREATE INDEX deliveries_endpoint ON deliveries(endpoint_id,created_at DESC,id);
CREATE INDEX deliveries_status ON deliveries(status,created_at DESC,id);
CREATE TABLE delivery_attempts (
 id text PRIMARY KEY, delivery_id text NOT NULL REFERENCES deliveries(id), generation integer NOT NULL,
 attempt integer NOT NULL, status_code integer NOT NULL, duration_ms bigint NOT NULL,
 error text NOT NULL DEFAULT '', created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(delivery_id,generation,attempt)
);
CREATE TABLE dead_letters (
 delivery_id text PRIMARY KEY REFERENCES deliveries(id), reason text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now()
);
