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

func (r *PurchaseRepository) GetPurchasesByFund(ctx context.Context, fundID int) ([]domain.Purchase, error) {
	query := `
SELECT id, fund_id, payer_id, amount, description, created_at
FROM purchases
WHERE fund_id = $1
ORDER BY created_at DESC
`
	rows, err := r.DB.Query(ctx, query, fundID)
	if err != nil {
		return nil, err
	}
	var funds []domain.Purchase
	for rows.Next() {
		var tempPurchase domain.Purchase
		err = rows.Scan(&tempPurchase.ID, &tempPurchase.FundID, &tempPurchase.PayerID, &tempPurchase.Amount, &tempPurchase.Description, &tempPurchase.CreatedAt)
		if err != nil {
			return nil, err
		}
		funds = append(funds, tempPurchase)
	}
	return funds, nil
}

func (r *PurchaseRepository) CreatePurchase(ctx context.Context, purchase *domain.Purchase) error {
	query := `INSERT INTO purchases
(fund_id, payer_id, amount, description) 
VALUES ($1, $2, $3, $4)
`
	_, err := r.DB.Exec(ctx, query, purchase.FundID, purchase.PayerID, purchase.Amount, purchase.Description)
	return err
}
