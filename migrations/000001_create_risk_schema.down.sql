BEGIN;

DROP TABLE IF EXISTS outbox_events;

DROP TABLE IF EXISTS merchant_blocklist_entries;
DROP TABLE IF EXISTS card_blocklist_entries;
DROP TABLE IF EXISTS customer_blocklist_entries;

DROP TABLE IF EXISTS customer_profile_country_stats;
DROP TABLE IF EXISTS customer_profile_mcc_stats;
DROP TABLE IF EXISTS customer_profiles;

DROP TABLE IF EXISTS fraud_labels;
DROP TABLE IF EXISTS authorization_outcomes;

DROP TABLE IF EXISTS decision_rule_hits;
DROP TABLE IF EXISTS risk_decisions;

DROP TABLE IF EXISTS rulesets;

DROP TABLE IF EXISTS authorization_attempts;

DROP TABLE IF EXISTS merchants;
DROP TABLE IF EXISTS cards;
DROP TABLE IF EXISTS customers;

DROP FUNCTION IF EXISTS prevent_hard_delete();

COMMIT;