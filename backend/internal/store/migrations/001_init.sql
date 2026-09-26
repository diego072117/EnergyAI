CREATE TABLE meters (
    id          BIGSERIAL PRIMARY KEY,
    meter_id    TEXT        NOT NULL UNIQUE,
    name        TEXT        NOT NULL,
    location    TEXT        NOT NULL,
    status      TEXT        NOT NULL DEFAULT 'OK',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE readings (
    id               BIGSERIAL PRIMARY KEY,
    meter_id         TEXT             NOT NULL REFERENCES meters (meter_id),
    timestamp        TIMESTAMPTZ      NOT NULL,
    consumption_kwh  DOUBLE PRECISION NOT NULL,
    voltage_v        DOUBLE PRECISION NOT NULL,
    current_a        DOUBLE PRECISION NOT NULL,
    power_factor     DOUBLE PRECISION NOT NULL,
    status           TEXT             NOT NULL DEFAULT 'OK',
    UNIQUE (meter_id, timestamp)
);

CREATE TABLE events (
    id          BIGSERIAL PRIMARY KEY,
    meter_id    TEXT        NOT NULL REFERENCES meters (meter_id),
    timestamp   TIMESTAMPTZ NOT NULL,
    type        TEXT        NOT NULL,
    description TEXT        NOT NULL DEFAULT ''
);
CREATE INDEX events_meter_idx ON events (meter_id, timestamp);

CREATE TABLE analysis_runs (
    id            TEXT        PRIMARY KEY,
    status        TEXT        NOT NULL,
    current_step  TEXT        NOT NULL DEFAULT '',
    steps         JSONB       NOT NULL DEFAULT '[]',
    summary       JSONB,
    error         TEXT        NOT NULL DEFAULT '',
    started_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at   TIMESTAMPTZ
);
CREATE INDEX analysis_runs_started_idx ON analysis_runs (started_at DESC);

CREATE TABLE anomalies (
    id                  BIGSERIAL PRIMARY KEY,
    analysis_id         TEXT             NOT NULL REFERENCES analysis_runs (id) ON DELETE CASCADE,
    meter_id            TEXT             NOT NULL REFERENCES meters (meter_id),
    detected_at         TIMESTAMPTZ      NOT NULL,
    type                TEXT             NOT NULL,
    is_anomaly          BOOLEAN          NOT NULL,
    severity            TEXT             NOT NULL,
    confidence          DOUBLE PRECISION NOT NULL,
    priority_score      DOUBLE PRECISION NOT NULL,
    reason              TEXT             NOT NULL,
    recommended_action  TEXT             NOT NULL,
    evidence_summary    JSONB            NOT NULL DEFAULT '[]',
    status              TEXT             NOT NULL DEFAULT 'OPEN',
    note                TEXT             NOT NULL DEFAULT '',
    window_start        TIMESTAMPTZ      NOT NULL,
    window_end          TIMESTAMPTZ      NOT NULL,
    evidence            JSONB            NOT NULL,
    explanation_source  TEXT             NOT NULL,
    created_at          TIMESTAMPTZ      NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ      NOT NULL DEFAULT now()
);
CREATE INDEX anomalies_analysis_idx ON anomalies (analysis_id, priority_score DESC);
CREATE INDEX anomalies_meter_idx ON anomalies (meter_id);

CREATE TABLE users (
    id             BIGSERIAL PRIMARY KEY,
    email          TEXT        NOT NULL UNIQUE,
    name           TEXT        NOT NULL,
    password_hash  TEXT        NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
