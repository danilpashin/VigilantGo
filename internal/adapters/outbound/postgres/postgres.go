package postgres

import (
	"context"

	authorizationdomain "vigilantgo/internal/authorization/domain"
	authorizationports "vigilantgo/internal/authorization/ports"
	profiledomain "vigilantgo/internal/profile/domain"
	profileports "vigilantgo/internal/profile/ports"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ProfileRepository struct {
	pool *pgxpool.Pool
}

func NewProfileRepository(pool *pgxpool.Pool) *ProfileRepository {
	return &ProfileRepository{pool: pool}
}

var _ profileports.ProfileRepository = (*ProfileRepository)(nil)

func (r *ProfileRepository) GetProfile(ctx context.Context, cardToken, currency string) (*profiledomain.Profile, error) {
	const query = `
		SELECT
			p.customer_id,
			p.currency,
			p.profile_version,
			p.successful_transaction_count,
			p.total_amount_minor,
			p.mean_amount_minor::text,
			p.amount_m2::text,
			p.first_successful_at,
			p.last_successful_at,
			p.updated_at
		FROM cards AS c
		JOIN customer_profiles AS p ON p.customer_id = c.customer_id
		WHERE c.token = $1
		  AND p.currency = $2
	`

	var profile profiledomain.Profile
	err := r.pool.QueryRow(ctx, query, cardToken, currency).Scan(
		&profile.CustomerID,
		&profile.Currency,
		&profile.ProfileVersion,
		&profile.SuccessfulTransactionCount,
		&profile.TotalAmountMinor,
		&profile.MeanAmountMinor,
		&profile.AmountM2,
		&profile.FirstSuccessfulAt,
		&profile.LastSuccessfulAt,
		&profile.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &profile, nil
}

func (r *ProfileRepository) SetProfile(ctx context.Context, profile profiledomain.Profile) (bool, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)

	const profileQuery = `
		INSERT INTO customer_profiles (
			customer_id,
			currency,
			profile_version,
			successful_transaction_count,
			total_amount_minor,
			mean_amount_minor,
			amount_m2,
			first_successful_at,
			last_successful_at,
			updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (customer_id, currency) DO UPDATE SET
			profile_version = EXCLUDED.profile_version,
			successful_transaction_count = EXCLUDED.successful_transaction_count,
			total_amount_minor = EXCLUDED.total_amount_minor,
			mean_amount_minor = EXCLUDED.mean_amount_minor,
			amount_m2 = EXCLUDED.amount_m2,
			first_successful_at = EXCLUDED.first_successful_at,
			last_successful_at = EXCLUDED.last_successful_at,
			updated_at = EXCLUDED.updated_at
	`

	if _, err = tx.Exec(ctx, profileQuery,
		profile.CustomerID,
		profile.Currency,
		profile.ProfileVersion,
		profile.SuccessfulTransactionCount,
		profile.TotalAmountMinor,
		profile.MeanAmountMinor,
		profile.AmountM2,
		profile.FirstSuccessfulAt,
		profile.LastSuccessfulAt,
		profile.UpdatedAt,
	); err != nil {
		return false, err
	}

	const mccQuery = `
		INSERT INTO customer_profile_mcc_stats (
			customer_id,
			currency,
			mcc,
			transaction_count,
			total_amount_minor,
			last_seen_at,
			updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (customer_id, currency, mcc) DO UPDATE SET
			transaction_count = EXCLUDED.transaction_count,
			total_amount_minor = EXCLUDED.total_amount_minor,
			last_seen_at = EXCLUDED.last_seen_at,
			updated_at = EXCLUDED.updated_at
	`

	for _, stat := range profile.MCCStats {
		if _, err = tx.Exec(ctx, mccQuery,
			profile.CustomerID,
			profile.Currency,
			stat.MCC,
			stat.TransactionCount,
			stat.TotalAmountMinor,
			stat.LastSeenAt,
			stat.UpdatedAt,
		); err != nil {
			return false, err
		}
	}

	const countryQuery = `
		INSERT INTO customer_profile_country_stats (
			customer_id,
			currency,
			country,
			transaction_count,
			total_amount_minor,
			last_seen_at,
			updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (customer_id, currency, country) DO UPDATE SET
			transaction_count = EXCLUDED.transaction_count,
			total_amount_minor = EXCLUDED.total_amount_minor,
			last_seen_at = EXCLUDED.last_seen_at,
			updated_at = EXCLUDED.updated_at
	`

	for _, stat := range profile.CountryStats {
		if _, err = tx.Exec(ctx, countryQuery,
			profile.CustomerID,
			profile.Currency,
			stat.Country,
			stat.TransactionCount,
			stat.TotalAmountMinor,
			stat.LastSeenAt,
			stat.UpdatedAt,
		); err != nil {
			return false, err
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return false, err
	}

	return true, nil
}

type OutcomeRepository struct {
	pool *pgxpool.Pool
}

func NewOutcomeRepository(pool *pgxpool.Pool) *OutcomeRepository {
	return &OutcomeRepository{pool: pool}
}

var _ authorizationports.OutcomeRepository = (*OutcomeRepository)(nil)

func (r *OutcomeRepository) SaveOutcome(ctx context.Context, outcome authorizationdomain.AuthorizationOutcome) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	const query = `
		INSERT INTO authorization_outcomes (
			authorization_id,
			status,
			processor_response_code,
			source,
			source_event_id,
			idempotency_key,
			occurred_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	if _, err = tx.Exec(ctx, query,
		outcome.AuthorizationID,
		outcome.Status,
		outcome.ProcessorResponseCode,
		outcome.Source,
		outcome.SourceEventID,
		outcome.IdempotencyKey,
		outcome.OccurredAt,
	); err != nil {
		return err
	}

	if err = tx.Commit(ctx); err != nil {
		return err
	}

	return nil
}
