package postgres

import (
	"context"
	"errors"
	"log/slog"

	"github.com/ganfay/split-core/internal/domain"
	"github.com/jackc/pgx/v5"

	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepository struct {
	DB *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	slog.Info("init userRepository")
	return &UserRepository{DB: db}
}

func (r *UserRepository) GetOrCreateRealUser(ctx context.Context, tgID *int64, username, firstName string) (int64, error) {
	var id int64
	query := `
	INSERT INTO app.users (tg_id, username, first_name, is_virtual) 
	VALUES ($1, $2, $3, false)
	ON CONFLICT (tg_id) DO UPDATE 
		SET username = EXCLUDED.username, first_name = EXCLUDED.first_name
	RETURNING id`
	err := r.DB.QueryRow(ctx, query, *tgID, username, firstName).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return id, nil
	} else if err != nil {
		return id, err
	}
	return id, nil
}

func (r *UserRepository) CreateVirtualUser(ctx context.Context, firstName string) (int64, error) {
	var id int64
	query := `
	INSERT INTO app.users (first_name, is_virtual) 
	VALUES ($1, true)
	RETURNING id`
	err := r.DB.QueryRow(ctx, query, firstName).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return id, nil
	} else if err != nil {
		return id, err
	}
	return id, nil
}

func (r *UserRepository) GetUserByIID(ctx context.Context, iID int64) (*domain.User, error) {
	var u domain.User
	query := `
SELECT tg_id, username, first_name, is_virtual, created_at FROM app.users WHERE id = $1`
	err := r.DB.QueryRow(ctx, query, iID).Scan(&u.TgID, &u.Username, &u.FirstName, &u.IsVirtual, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepository) DeleteUser(ctx context.Context, iID int64) error {
	query := `DELETE FROM app.users WHERE id = $1`
	_, err := r.DB.Exec(ctx, query, iID)
	return err
}
