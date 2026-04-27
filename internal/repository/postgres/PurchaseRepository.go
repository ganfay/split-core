package postgres

import (
	"SplitCore/internal/domain"
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PurchaseRepository struct {
	DB *pgxpool.Pool
}

func NewPurchaseRepository(pool *pgxpool.Pool) *PurchaseRepository {
	slog.Info("init purchaseRepository")
	return &PurchaseRepository{DB: pool}
}

func (r *PurchaseRepository) GetPurchasesByFundPagination(ctx context.Context, fundID int, limit int, offset int) ([]domain.Purchase, error) {
	query := `
SELECT p.id, p.fund_id, p.payer_id, u.username, u.first_name, p.amount, p.description, p.created_at
FROM app.purchases p 
JOIN app.users u ON p.payer_id = u.tg_id
WHERE p.fund_id = $1
ORDER BY created_at DESC
OFFSET $2 LIMIT $3
`
	rows, err := r.DB.Query(ctx, query, fundID, offset, limit)
	if err != nil {
		return nil, err
	}
	var funds []domain.Purchase
	for rows.Next() {
		var tempPurchase domain.Purchase
		err = rows.Scan(&tempPurchase.ID, &tempPurchase.FundID, &tempPurchase.Payer.TgID, &tempPurchase.Payer.Username, &tempPurchase.Payer.FirstName, &tempPurchase.Amount, &tempPurchase.Description, &tempPurchase.CreatedAt)
		if err != nil {
			return nil, err
		}
		funds = append(funds, tempPurchase)
	}
	return funds, nil
}

func (r *PurchaseRepository) GetPurchasesByFundAll(ctx context.Context, fundID int) ([]domain.Purchase, error) {
	query := `
SELECT p.id, p.fund_id, p.payer_id, u.username, u.first_name, p.amount, p.description, p.created_at
FROM app.purchases p 
JOIN app.users u ON p.payer_id = u.tg_id
WHERE p.fund_id = $1
ORDER BY created_at DESC
`
	rows, err := r.DB.Query(ctx, query, fundID)
	if err != nil {
		return nil, err
	}
	var funds []domain.Purchase
	for rows.Next() {
		var tempPurchase domain.Purchase
		err = rows.Scan(&tempPurchase.ID, &tempPurchase.FundID, &tempPurchase.Payer.TgID, &tempPurchase.Payer.Username, &tempPurchase.Payer.FirstName, &tempPurchase.Amount, &tempPurchase.Description, &tempPurchase.CreatedAt)
		if err != nil {
			return nil, err
		}
		funds = append(funds, tempPurchase)
	}
	return funds, nil
}

func (r *PurchaseRepository) CreatePurchase(ctx context.Context, purchase *domain.Purchase) error {
	query := `INSERT INTO app.purchases
(fund_id, payer_id, amount, description) 
VALUES ($1, $2, $3, $4)
`
	_, err := r.DB.Exec(ctx, query, purchase.FundID, purchase.Payer.TgID, purchase.Amount, purchase.Description)
	return err
}
