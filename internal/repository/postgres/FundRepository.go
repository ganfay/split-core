package postgres

import (
	"SplitCore/internal/domain"
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

type FundRepository struct {
	DB *pgxpool.Pool
}

func NewFundRepository(pool *pgxpool.Pool) *FundRepository {
	slog.Info("init fundRepository")

	return &FundRepository{DB: pool}
}

func (r *FundRepository) Create(ctx context.Context, fund *domain.Fund) (*domain.Fund, error) {

	err := r.DB.QueryRow(ctx, `INSERT INTO funds
    (name, author_id, invite_code) 
	VALUES ($1, $2, $3) 
	ON CONFLICT DO NOTHING
	RETURNING id, created_at`, fund.Name, fund.AuthorID, fund.InviteCode).Scan(&fund.ID, &fund.CreatedAt)
	slog.Info("create fund", "fund", fund, "ctx", ctx)

	return fund, err
}

func (r *FundRepository) GetByInviteCode(ctx context.Context, code string) (*domain.Fund, error) {
	var fund domain.Fund
	err := r.DB.QueryRow(ctx, `SELECT (id, name, author_id, invite_code, created_at) FROM funds WHERE invite_code = $1`, code).Scan(
		&fund.ID, &fund.Name, &fund.AuthorID, &fund.InviteCode, &fund.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &fund, nil
}

func (r *FundRepository) GetByUserID(ctx context.Context, userID int64, limit string, offset string) ([]domain.Fund, error) {
	var funds []domain.Fund
	query, err := r.DB.Query(ctx, `SELECT id, name, author_id, invite_code, created_at FROM funds WHERE author_id = $1 LIMIT $2 OFFSET $3`, userID, limit, offset)
	if err != nil {
		slog.Debug(err.Error())
		return nil, err
	}
	for query.Next() {
		var fund domain.Fund
		err = query.Scan(&fund.ID, &fund.Name, &fund.AuthorID, &fund.InviteCode, &fund.CreatedAt)
		if err != nil {
			slog.Warn(err.Error())
			return nil, err
		}
		funds = append(funds, fund)
	}
	defer query.Close()
	return funds, nil
}
