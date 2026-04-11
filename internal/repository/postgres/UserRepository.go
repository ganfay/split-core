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

func (r *UserRepository) Create(ctx context.Context, u *domain.User) (*domain.User, error) {
	err := r.DB.QueryRow(ctx, `INSERT INTO users (tg_id, username, first_name) 
	VALUES ($1, $2, $3)
	ON CONFLICT (tg_id) DO NOTHING 	
	RETURNING created_at`, u.TgID, u.Username, u.FirstName).Scan(&u.CreatedAt)
	return u, err
}
