package postgres

import (
	"SplitCore/internal/domain"
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5"
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
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func(tx pgx.Tx, ctx context.Context) {
		err := tx.Rollback(ctx)
		if err != nil {
			slog.Debug("rollback", "err", err)
			return
		}
	}(tx, ctx)

	err = r.DB.QueryRow(ctx, `INSERT INTO funds
    (name, author_id, invite_code) 
	VALUES ($1, $2, $3) 
	ON CONFLICT DO NOTHING
	RETURNING id, created_at`, fund.Name, fund.AuthorID, fund.InviteCode).Scan(&fund.ID, &fund.CreatedAt)
	if err != nil {
		return nil, err
	}
	queryMember := `INSERT INTO fund_members (fund_id, user_id) VALUES ($1, $2)`
	_, err = tx.Exec(ctx, queryMember, fund.ID, fund.AuthorID)
	if err != nil {
		return nil, err
	}
	err = tx.Commit(ctx)
	if err != nil {
		return nil, err
	}
	return fund, err
}

func (r *FundRepository) GetInfo(ctx context.Context, reqFund *domain.Fund) (*domain.Fund, error) {
	var fund domain.Fund
	query := `
		SELECT id, name, author_id, invite_code, created_at 
		FROM funds 
		WHERE id = $1 OR (invite_code = $2 AND $2 <> '') 
		LIMIT 1`

	err := r.DB.QueryRow(ctx, query, reqFund.ID, reqFund.InviteCode).Scan(
		&fund.ID, &fund.Name, &fund.AuthorID, &fund.InviteCode, &fund.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &fund, nil
}

func (r *FundRepository) GetByUserID(ctx context.Context, userID int64, limit int, offset int) ([]domain.Fund, error) {
	query := `
        SELECT f.id, f.name, f.author_id, f.invite_code, f.created_at
        FROM funds f
        JOIN fund_members fm ON f.id = fm.fund_id
        WHERE fm.user_id = $1
        ORDER BY f.created_at DESC
        LIMIT $2 OFFSET $3`

	allFunds, err := r.DB.Query(ctx, query, userID, limit, offset)
	if err != nil {
		slog.Debug(err.Error())
		return nil, err
	}
	var funds []domain.Fund
	for allFunds.Next() {
		var fund domain.Fund
		err = allFunds.Scan(&fund.ID, &fund.Name, &fund.AuthorID, &fund.InviteCode, &fund.CreatedAt)
		if err != nil {
			return nil, err
		}
		funds = append(funds, fund)
	}
	defer allFunds.Close()
	return funds, nil
}

func (r *FundRepository) AddMember(ctx context.Context, fund *domain.Fund, userID int64) error {
	queryMember := `INSERT INTO fund_members (fund_id, user_id) VALUES ($1, $2)`
	_, err := r.DB.Exec(ctx, queryMember, fund.ID, userID)
	return err
}

func (r *FundRepository) GetPurchasesByFund(ctx context.Context, fund *domain.Fund) ([]domain.Purchase, error) {
	query := `
SELECT id, fund_id, payer_id, amount, description, created_at
FROM purchases
WHERE fund_id = $1
ORDER BY created_at DESC
`
	rows, err := r.DB.Query(ctx, query, fund.ID)
	if err != nil {
		return nil, err
	}
	var funds []domain.Purchase
	for rows.Next() {
		var tempFund domain.Purchase
		err = rows.Scan(&tempFund.ID, &tempFund.FundID, &tempFund.PayerID, &tempFund.Amount, &tempFund.Description, &tempFund.CreatedAt)
		if err != nil {
			return nil, err
		}
		funds = append(funds, tempFund)
	}
	return funds, nil
}

func (r *FundRepository) CreatePurchase(ctx context.Context, purchase *domain.Purchase) error {
	query := `INSERT INTO purchases
(fund_id, payer_id, amount, description) 
VALUES ($1, $2, $3, $4)
`
	_, err := r.DB.Exec(ctx, query, purchase.FundID, purchase.PayerID, purchase.Amount, purchase.Description)
	return err
}
