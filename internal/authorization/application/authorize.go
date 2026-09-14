package application

import (
	"context"
	"errors"
	"time"
	"uuid"

	domain "vigilantgo/internal/authorization/domain"
	"vigilantgo/internal/authorization/ports"
	"vigilantgo/internal/platform/database"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalidCommand             = errors.New("invalid authorization command")
	ErrProfileReaderMissing       = errors.New("authorization profile reader is not configured")
	ErrOutcomeWriterMissing       = errors.New("authorization outcome writer is not configured")
	ErrTransactionManagerMissing  = errors.New("authorization transaction manager is not configured")
	ErrTransactionCallbackMissing = errors.New("authorization transaction callback is not configured")
)

type TransactionManager struct {
	pool *pgxpool.Pool
}

func NewTransactionManager(pool *pgxpool.Pool) *TransactionManager {
	return &TransactionManager{pool: pool}
}

func (m *TransactionManager) WithinTransaction(ctx context.Context, fn func(context.Context) error) (err error) {
	if m == nil || m.pool == nil {
		return ErrTransactionManagerMissing
	}
	if fn == nil {
		return ErrTransactionCallbackMissing
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	tx, err := m.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() {
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
			err = errors.Join(err, rollbackErr)
		}
	}()

	err = fn(database.WithTransaction(ctx, tx))
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type Service struct {
	profileReader      ports.ProfileReader
	outcomeWriter      ports.OutcomeWriter
	transactionManager *TransactionManager
}

func NewService(profileReader ports.ProfileReader, outcomeWriter ports.OutcomeWriter, transactionManager ...*TransactionManager) *Service {
	service := &Service{profileReader: profileReader, outcomeWriter: outcomeWriter}
	if len(transactionManager) > 0 {
		service.transactionManager = transactionManager[0]
	}
	return service
}

func (s *Service) Evaluate(ctx context.Context, cmd AuthorizeCommand) (AuthorizationResult, error) {
	startedAt := time.Now()

	if err := ctx.Err(); err != nil {
		return AuthorizationResult{}, err
	}
	if cmd.AuthorizationID == "" || cmd.SourceSystem == "" || cmd.AmountMinor == 0 {
		return AuthorizationResult{}, ErrInvalidCommand
	}
	if s.profileReader == nil {
		return AuthorizationResult{}, ErrProfileReaderMissing
	}

	transaction := domain.Transaction{
		AmountMinor:     cmd.AmountMinor,
		Currency:        cmd.Currency,
		MerchantMCC:     cmd.Merchant.MCC,
		MerchantCountry: cmd.Merchant.Country,
		Channel:         cmd.Channel,
		CardPresent:     cmd.CardPresent,
		IPCountry:       cmd.IPCountry,
	}

	assessment := domain.EvaluateRisk(transaction)
	decision := domain.Decide(assessment, time.Now().UTC())

	return AuthorizationResult{
		DecisionID:      uuid.New().String(),
		AuthorizationID: cmd.AuthorizationID,
		Verdict:         string(decision.Verdict),
		RiskScore:       decision.Assessment.Score,
		ReasonCodes:     decision.Assessment.ReasonCodes,
		RulesetVersion:  decision.RulesetVersion,
		Degraded:        decision.Degraded,
		Replayed:        false,
		ProcessingTime:  time.Since(startedAt),
	}, nil
}

func (s *Service) SaveOutcome(ctx context.Context, cmd OutcomeCommand) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if cmd.AuthorizationID == "" || cmd.SourceSystem == "" || cmd.Status == "" || cmd.RequestID == "" || cmd.IdempotencyKey == "" {
		return ErrInvalidCommand
	}

	authorizationID, err := uuid.Parse(cmd.AuthorizationID)
	if err != nil {
		return ErrInvalidCommand
	}
	if s.outcomeWriter == nil {
		return ErrOutcomeWriterMissing
	}

	var processorResponseCode *string
	if cmd.ProccessorResponseCode != "" {
		processorResponseCode = &cmd.ProccessorResponseCode
	}

	outcome := domain.AuthorizationOutcome{
		AuthorizationID:       authorizationID,
		Status:                domain.OutcomeStatus(cmd.Status),
		ProcessorResponseCode: processorResponseCode,
		Source:                cmd.SourceSystem,
		SourceEventID:         cmd.RequestID,
		IdempotencyKey:        cmd.IdempotencyKey,
		OccurredAt:            cmd.OccurredAt,
	}

	save := func(saveCtx context.Context) error {
		return s.outcomeWriter.SaveOutcome(saveCtx, outcome)
	}
	if s.transactionManager != nil {
		return s.transactionManager.WithinTransaction(ctx, save)
	}
	return save(ctx)
}
