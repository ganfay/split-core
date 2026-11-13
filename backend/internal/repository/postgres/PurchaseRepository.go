package postgres

import (
	"context"
	"database/sql"
	"log/slog"

	"github.com/ganfay/split-core/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PurchaseRepository struct {
	DB *pgxpool.Pool
}

func NewPurchaseRepository(pool *pgxpool.Pool) *PurchaseRepository {
	slog.Info("init purchaseRepository")
	return &PurchaseRepository{DB: pool}
}

func scanPurchases(rows pgx.Rows) ([]domain.Purchase, error) {
	purchases := make([]domain.Purchase, 0)
	for rows.Next() {
		var p domain.Purchase
		var username sql.NullString
		var description sql.NullString
		var isVirtual sql.NullBool
		err := rows.Scan(
			&p.ID,
			&p.FundID,
			&p.Payer.ID,
			&p.Payer.TgID,
			&username,
			&p.Payer.FirstName,
			&isVirtual,
			&p.Amount,
			&description,
			&p.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		p.Payer.Username = username.String
		p.Payer.IsVirtual = isVirtual.Bool
		p.Description = description.String
		purchases = append(purchases, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return purchases, nil
}

func (r *PurchaseRepository) GetPurchasesByFundPagination(ctx context.Context, fundID int, limit int, offset int) ([]domain.Purchase, error) {
	query := `
SELECT p.id, p.fund_id, p.payer_id, u.tg_id, u.username, u.first_name, u.is_virtual, p.amount, p.description, p.created_at
FROM app.purchases p 
JOIN app.users u ON p.payer_id = u.id
WHERE p.fund_id = $1
ORDER BY p.created_at DESC
OFFSET $2 LIMIT $3
`
	rows, err := r.DB.Query(ctx, query, fundID, offset, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPurchases(rows)
}

func (r *PurchaseRepository) GetPurchasesByFundAll(ctx context.Context, fundID int) ([]domain.Purchase, error) {
	query := `
SELECT p.id, p.fund_id, p.payer_id, u.tg_id, u.username, u.first_name, u.is_virtual, p.amount, p.description, p.created_at
FROM app.purchases p 
JOIN app.users u ON p.payer_id = u.id
WHERE p.fund_id = $1
ORDER BY p.created_at DESC
`
	rows, err := r.DB.Query(ctx, query, fundID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPurchases(rows)
}

func (r *PurchaseRepository) CreatePurchase(ctx context.Context, fundID int, amount float64, IID int64, desc string) error {
	query := `INSERT INTO app.purchases
(fund_id, payer_id, amount, description) 
VALUES ($1, $2, $3, $4)
`
	_, err := r.DB.Exec(ctx, query, fundID, IID, amount, desc)
	return err
}
