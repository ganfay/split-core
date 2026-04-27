package postgres

import (
	"SplitCore/internal/domain"
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepository struct {
	DB *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	slog.Info("init userRepository")
	return &UserRepository{DB: db}
}

func (r *UserRepository) CreateUser(ctx context.Context, u *domain.User) (*domain.User, error) {
	err := r.DB.QueryRow(ctx, `INSERT INTO app.users (tg_id, username, first_name) 
	VALUES ($1, $2, $3)
	ON CONFLICT (tg_id) DO NOTHING 	
	RETURNING created_at`, u.TgID, u.Username, u.FirstName).Scan(&u.CreatedAt)
	return u, err
}

func (r *UserRepository) GetUser(ctx context.Context, tgID int64) (*domain.User, error) {
	var u domain.User
	u.TgID = tgID
	err := r.DB.QueryRow(ctx, `SELECT username, first_name, created_at FROM app.users WHERE tg_id = $1`, tgID).Scan(&u.Username, &u.FirstName, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}
