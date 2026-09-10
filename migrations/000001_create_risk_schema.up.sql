BEGIN;

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE FUNCTION prevent_hard_delete()
RETURNS trigger
LANGUAGE plpgsql
AS $$BEGIN
    RAISE EXCEPTION 'hard delete from table % is prohibited', TG_TABLE_NAME
        USING ERRCODE = '55000';
END;$$;

-- ---------------------------------------------------------------------------
-- Customers, cards, merchants
-- ---------------------------------------------------------------------------

CREATE TABLE customers (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    external_ref    text NOT NULL UNIQUE,
    status          text NOT NULL,
    home_country    char(2),
    home_city       text,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT customers_status_check
        CHECK (status IN ('ACTIVE', 'RESTRICTED', 'CLOSED')),
    CONSTRAINT customers_home_country_check
        CHECK (
            home_country IS NULL
            OR home_country ~ '^[A-Z]{2}$'
        )
);

CREATE TABLE cards (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id         uuid NOT NULL REFERENCES customers(id),
    token               text NOT NULL UNIQUE,
    pan_fingerprint     bytea UNIQUE,
    last4               char(4) NOT NULL,
    status              text NOT NULL,
    expires_at          timestamptz,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT cards_last4_check
        CHECK (last4 ~ '^[0-9]{4}$'),
    CONSTRAINT cards_status_check
        CHECK (status IN ('ACTIVE', 'BLOCKED', 'EXPIRED', 'CLOSED'))
);

CREATE INDEX cards_customer_id_idx
    ON cards (customer_id);

CREATE TABLE merchants (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    external_id     text NOT NULL UNIQUE,
    name            text NOT NULL,
    mcc             char(4) NOT NULL,
    country         char(2) NOT NULL,
    city            text,
    risk_status     text NOT NULL DEFAULT 'KNOWN',
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT merchants_mcc_check
        CHECK (mcc ~ '^[0-9]{4}$'),
    CONSTRAINT merchants_country_check
        CHECK (country ~ '^[A-Z]{2}$'),
    CONSTRAINT merchants_risk_status_check
        CHECK (risk_status IN ('KNOWN', 'VERIFIED', 'HIGH_RISK', 'BLOCKED'))
);

-- ---------------------------------------------------------------------------
-- Authorization attempts
-- ---------------------------------------------------------------------------

CREATE TABLE authorization_attempts (
    id                          uuid PRIMARY KEY DEFAULT gen_random_uuid(),

    source_system               text NOT NULL,
    external_authorization_id   text NOT NULL,
    idempotency_key             text NOT NULL,
    request_hash                bytea NOT NULL,

    customer_id                 uuid NOT NULL REFERENCES customers(id),
    card_id                     uuid NOT NULL REFERENCES cards(id),
    merchant_id                 uuid REFERENCES merchants(id),

    merchant_external_id        text,
    merchant_name               text,
    merchant_mcc                char(4),
    merchant_country            char(2),
    merchant_city               text,
    merchant_terminal           text,

    occurred_at                 timestamptz NOT NULL,
    received_at                 timestamptz NOT NULL DEFAULT now(),

    amount_minor                bigint NOT NULL,
    currency                    char(3) NOT NULL,

    channel                     text NOT NULL,
    pos_entry_mode              text,
    card_present                boolean,
    device_id                   text,
    ip_country                  char(2),

    transport_type              text,
    stan                        text,
    rrn                         text,

    created_at                  timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT authorization_attempts_source_external_uk
        UNIQUE (source_system, external_authorization_id),

    CONSTRAINT authorization_attempts_source_idempotency_uk
        UNIQUE (source_system, idempotency_key),

    CONSTRAINT authorization_attempts_source_request_hash_uk
        UNIQUE (source_system, request_hash),

    CONSTRAINT authorization_attempts_amount_minor_check
        CHECK (amount_minor > 0),

    CONSTRAINT authorization_attempts_currency_check
        CHECK (currency ~ '^[A-Z]{3}$'),

    CONSTRAINT authorization_attempts_merchant_mcc_check
        CHECK (
            merchant_mcc IS NULL
            OR merchant_mcc ~ '^[0-9]{4}$'
        ),

    CONSTRAINT authorization_attempts_merchant_country_check
        CHECK (
            merchant_country IS NULL
            OR merchant_country ~ '^[A-Z]{2}$'
        ),

    CONSTRAINT authorization_attempts_ip_country_check
        CHECK (
            ip_country IS NULL
            OR ip_country ~ '^[A-Z]{2}$'
        ),

    CONSTRAINT authorization_attempts_time_check
        CHECK (received_at >= occurred_at)
);

CREATE INDEX authorization_attempts_card_occurred_idx
    ON authorization_attempts (card_id, occurred_at DESC);

CREATE INDEX authorization_attempts_customer_occurred_idx
    ON authorization_attempts (customer_id, occurred_at DESC);

CREATE INDEX authorization_attempts_merchant_occurred_idx
    ON authorization_attempts (merchant_id, occurred_at DESC)
    WHERE merchant_id IS NOT NULL;

CREATE INDEX authorization_attempts_merchant_external_occurred_idx
    ON authorization_attempts (merchant_external_id, occurred_at DESC)
    WHERE merchant_external_id IS NOT NULL;

CREATE INDEX authorization_attempts_occurred_at_idx
    ON authorization_attempts (occurred_at DESC);

-- ---------------------------------------------------------------------------
-- Risk decisions and rule hits
-- ---------------------------------------------------------------------------

CREATE TABLE risk_decisions (
    id                      uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    authorization_id        uuid NOT NULL UNIQUE
                                REFERENCES authorization_attempts(id),
    verdict                 text NOT NULL,
    risk_score              smallint NOT NULL,
    ruleset_version         text NOT NULL,
    degraded                boolean NOT NULL DEFAULT false,
    degradation_reason      text,
    processing_time_ms      integer NOT NULL,
    created_at              timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT risk_decisions_verdict_check
        CHECK (verdict IN ('ALLOW', 'REVIEW', 'BLOCK')),

    CONSTRAINT risk_decisions_risk_score_check
        CHECK (risk_score BETWEEN 0 AND 1000),

    CONSTRAINT risk_decisions_processing_time_check
        CHECK (processing_time_ms >= 0),

    CONSTRAINT risk_decisions_degradation_reason_check
        CHECK (
            (degraded AND degradation_reason IS NOT NULL)
            OR
            (NOT degraded AND degradation_reason IS NULL)
        ),

    CONSTRAINT risk_decisions_degradation_reason_length_check
        CHECK (
            degradation_reason IS NULL
            OR octet_length(degradation_reason) <= 1024
        )
);

CREATE TABLE decision_rule_hits (
    decision_id        uuid NOT NULL REFERENCES risk_decisions(id),
    sequence_no        integer NOT NULL,
    rule_code          text NOT NULL,
    score_delta        integer NOT NULL DEFAULT 0,
    hard_action        text,
    metadata           jsonb NOT NULL DEFAULT '{}'::jsonb,

    PRIMARY KEY (decision_id, sequence_no),

    CONSTRAINT decision_rule_hits_rule_uk
        UNIQUE (decision_id, rule_code),

    CONSTRAINT decision_rule_hits_sequence_check
        CHECK (sequence_no >= 0),

    CONSTRAINT decision_rule_hits_hard_action_check
        CHECK (
            hard_action IS NULL
            OR hard_action IN ('ALLOW', 'REVIEW', 'BLOCK')
        ),

    CONSTRAINT decision_rule_hits_metadata_object_check
        CHECK (jsonb_typeof(metadata) = 'object'),

    CONSTRAINT decision_rule_hits_metadata_size_check
        CHECK (octet_length(metadata::text) <= 8192)
);

-- ---------------------------------------------------------------------------
-- Authorization outcomes and fraud labels
-- ---------------------------------------------------------------------------

CREATE TABLE authorization_outcomes (
    id                      uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    authorization_id        uuid NOT NULL REFERENCES authorization_attempts(id),
    status                  text NOT NULL,
    processor_response_code text,
    source                  text NOT NULL,
    source_event_id         text NOT NULL,
    idempotency_key         text NOT NULL,
    occurred_at             timestamptz NOT NULL,
    recorded_at             timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT authorization_outcomes_status_check
        CHECK (status IN ('AUTHORIZED', 'DECLINED', 'REVERSED')),

    CONSTRAINT authorization_outcomes_source_event_uk
        UNIQUE (source, source_event_id),

    CONSTRAINT authorization_outcomes_source_idempotency_uk
        UNIQUE (source, idempotency_key),

    CONSTRAINT authorization_outcomes_time_check
        CHECK (recorded_at >= occurred_at)
);

CREATE INDEX authorization_outcomes_authorization_occurred_idx
    ON authorization_outcomes (authorization_id, occurred_at DESC, recorded_at DESC);

CREATE TABLE fraud_labels (
    id                      uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    authorization_id        uuid NOT NULL REFERENCES authorization_attempts(id),
    label                   text NOT NULL,
    source                  text NOT NULL,
    reason                  text,
    analyst_reference       text,
    source_event_id         text,
    labeled_at              timestamptz NOT NULL,
    recorded_at             timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT fraud_labels_label_check
        CHECK (
            label IN (
                'UNKNOWN',
                'LEGITIMATE',
                'SUSPECTED_FRAUD',
                'CONFIRMED_FRAUD'
            )
        ),

    CONSTRAINT fraud_labels_source_event_uk
        UNIQUE NULLS NOT DISTINCT (source, source_event_id),

    CONSTRAINT fraud_labels_time_check
        CHECK (recorded_at >= labeled_at),

    CONSTRAINT fraud_labels_reason_length_check
        CHECK (reason IS NULL OR octet_length(reason) <= 2048),

    CONSTRAINT fraud_labels_analyst_reference_length_check
        CHECK (
            analyst_reference IS NULL
            OR octet_length(analyst_reference) <= 512
        )
);

CREATE INDEX fraud_labels_authorization_labeled_idx
    ON fraud_labels (authorization_id, labeled_at DESC, recorded_at DESC);

-- ---------------------------------------------------------------------------
-- Customer profiles
-- ---------------------------------------------------------------------------

CREATE TABLE customer_profiles (
    customer_id                 uuid NOT NULL REFERENCES customers(id),
    currency                    char(3) NOT NULL,
    profile_version             bigint NOT NULL DEFAULT 1,
    successful_transaction_count bigint NOT NULL DEFAULT 0,
    total_amount_minor          bigint NOT NULL DEFAULT 0,
    mean_amount_minor           numeric(30, 6) NOT NULL DEFAULT 0,
    amount_m2                   numeric(38, 6) NOT NULL DEFAULT 0,
    first_successful_at         timestamptz,
    last_successful_at          timestamptz,
    updated_at                  timestamptz NOT NULL DEFAULT now(),

    PRIMARY KEY (customer_id, currency),

    CONSTRAINT customer_profiles_currency_check
        CHECK (currency ~ '^[A-Z]{3}$'),

    CONSTRAINT customer_profiles_version_check
        CHECK (profile_version > 0),

    CONSTRAINT customer_profiles_count_check
        CHECK (successful_transaction_count >= 0),

    CONSTRAINT customer_profiles_total_check
        CHECK (total_amount_minor >= 0),

    CONSTRAINT customer_profiles_mean_check
        CHECK (mean_amount_minor >= 0),

    CONSTRAINT customer_profiles_m2_check
        CHECK (amount_m2 >= 0),

    CONSTRAINT customer_profiles_times_check
        CHECK (
            first_successful_at IS NULL
            OR last_successful_at IS NULL
            OR first_successful_at <= last_successful_at
        ),

    CONSTRAINT customer_profiles_empty_state_check
        CHECK (
            successful_transaction_count > 0
            OR (
                total_amount_minor = 0
                AND mean_amount_minor = 0
                AND amount_m2 = 0
                AND first_successful_at IS NULL
                AND last_successful_at IS NULL
            )
        )
);

CREATE TABLE customer_profile_mcc_stats (
    customer_id        uuid NOT NULL,
    currency           char(3) NOT NULL,
    mcc                char(4) NOT NULL,
    transaction_count  bigint NOT NULL DEFAULT 0,
    total_amount_minor bigint NOT NULL DEFAULT 0,
    last_seen_at       timestamptz,
    updated_at         timestamptz NOT NULL DEFAULT now(),

    PRIMARY KEY (customer_id, currency, mcc),

    FOREIGN KEY (customer_id, currency)
        REFERENCES customer_profiles(customer_id, currency),

    CONSTRAINT customer_profile_mcc_stats_mcc_check
        CHECK (mcc ~ '^[0-9]{4}$'),

    CONSTRAINT customer_profile_mcc_stats_count_check
        CHECK (transaction_count >= 0),

    CONSTRAINT customer_profile_mcc_stats_total_check
        CHECK (total_amount_minor >= 0)
);

CREATE INDEX customer_profile_mcc_stats_owner_idx
    ON customer_profile_mcc_stats (customer_id, currency);

CREATE TABLE customer_profile_country_stats (
    customer_id        uuid NOT NULL,
    currency           char(3) NOT NULL,
    country            char(2) NOT NULL,
    transaction_count  bigint NOT NULL DEFAULT 0,
    total_amount_minor bigint NOT NULL DEFAULT 0,
    last_seen_at       timestamptz,
    updated_at         timestamptz NOT NULL DEFAULT now(),

    PRIMARY KEY (customer_id, currency, country),

    FOREIGN KEY (customer_id, currency)
        REFERENCES customer_profiles(customer_id, currency),

    CONSTRAINT customer_profile_country_stats_country_check
        CHECK (country ~ '^[A-Z]{2}$'),

    CONSTRAINT customer_profile_country_stats_count_check
        CHECK (transaction_count >= 0),

    CONSTRAINT customer_profile_country_stats_total_check
        CHECK (total_amount_minor >= 0)
);

CREATE INDEX customer_profile_country_stats_owner_idx
    ON customer_profile_country_stats (customer_id, currency);

-- ---------------------------------------------------------------------------
-- Blocklists
-- ---------------------------------------------------------------------------

CREATE TABLE customer_blocklist_entries (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id     uuid NOT NULL REFERENCES customers(id),
    reason_code     text NOT NULL,
    comment         text,
    active_from     timestamptz NOT NULL DEFAULT now(),
    active_until    timestamptz,
    created_by      text NOT NULL,
    source          text NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    revoked_at      timestamptz,

    CONSTRAINT customer_blocklist_interval_check
        CHECK (active_until IS NULL OR active_until > active_from),

    CONSTRAINT customer_blocklist_revoked_check
        CHECK (revoked_at IS NULL OR revoked_at >= created_at),

    CONSTRAINT customer_blocklist_comment_length_check
        CHECK (comment IS NULL OR octet_length(comment) <= 2048)
);

CREATE INDEX customer_blocklist_active_idx
    ON customer_blocklist_entries (customer_id, active_from, active_until)
    WHERE revoked_at IS NULL;

CREATE TABLE card_blocklist_entries (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    card_id         uuid NOT NULL REFERENCES cards(id),
    reason_code     text NOT NULL,
    comment         text,
    active_from     timestamptz NOT NULL DEFAULT now(),
    active_until    timestamptz,
    created_by      text NOT NULL,
    source          text NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    revoked_at      timestamptz,

    CONSTRAINT card_blocklist_interval_check
        CHECK (active_until IS NULL OR active_until > active_from),

    CONSTRAINT card_blocklist_revoked_check
        CHECK (revoked_at IS NULL OR revoked_at >= created_at),

    CONSTRAINT card_blocklist_comment_length_check
        CHECK (comment IS NULL OR octet_length(comment) <= 2048)
);

CREATE INDEX card_blocklist_active_idx
    ON card_blocklist_entries (card_id, active_from, active_until)
    WHERE revoked_at IS NULL;

CREATE TABLE merchant_blocklist_entries (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    merchant_id     uuid NOT NULL REFERENCES merchants(id),
    reason_code     text NOT NULL,
    comment         text,
    active_from     timestamptz NOT NULL DEFAULT now(),
    active_until    timestamptz,
    created_by      text NOT NULL,
    source          text NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    revoked_at      timestamptz,

    CONSTRAINT merchant_blocklist_interval_check
        CHECK (active_until IS NULL OR active_until > active_from),

    CONSTRAINT merchant_blocklist_revoked_check
        CHECK (revoked_at IS NULL OR revoked_at >= created_at),

    CONSTRAINT merchant_blocklist_comment_length_check
        CHECK (comment IS NULL OR octet_length(comment) <= 2048)
);

CREATE INDEX merchant_blocklist_active_idx
    ON merchant_blocklist_entries (merchant_id, active_from, active_until)
    WHERE revoked_at IS NULL;

-- ---------------------------------------------------------------------------
-- Rulesets
-- ---------------------------------------------------------------------------

CREATE TABLE rulesets (
    version             text PRIMARY KEY,
    content_checksum    bytea NOT NULL UNIQUE,
    config_snapshot     jsonb NOT NULL,
    activated_at        timestamptz,
    deactivated_at      timestamptz,
    created_at          timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT rulesets_snapshot_object_check
        CHECK (jsonb_typeof(config_snapshot) = 'object'),

    CONSTRAINT rulesets_snapshot_size_check
        CHECK (octet_length(config_snapshot::text) <= 1048576),

    CONSTRAINT rulesets_activation_interval_check
        CHECK (
            deactivated_at IS NULL
            OR (
                activated_at IS NOT NULL
                AND deactivated_at > activated_at
            )
        )
);

COMMIT;