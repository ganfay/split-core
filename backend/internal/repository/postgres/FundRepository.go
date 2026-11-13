package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"github.com/ganfay/split-core/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type FundRepository struct {
	DB *pgxpool.Pool
}

func NewFundRepository(pool *pgxpool.Pool) *FundRepository {
	slog.Info("init fundRepository")

	return &FundRepository{DB: pool}
}

func (r *FundRepository) CreateFund(ctx context.Context, fund *domain.Fund) (*domain.Fund, error) {
	if fund == nil {
		return nil, fmt.Errorf("%w: fund is nil", domain.ErrInvalidInput)
	}
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		err := tx.Rollback(ctx)
		if err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			slog.Error("Failed to rollback transaction", slog.Any("err", err))
		}
	}()

	err = tx.QueryRow(ctx, `INSERT INTO app.funds
    (name, author_id, invite_code) 
	VALUES ($1, $2, $3) 
	ON CONFLICT DO NOTHING
	RETURNING id, created_at`, fund.Name, fund.AuthorID, fund.InviteCode).Scan(&fund.ID, &fund.CreatedAt)
	if err != nil {
		return nil, err
	}
	queryMember := `INSERT INTO app.fund_members (fund_id, user_id) VALUES ($1, $2)`
	_, err = tx.Exec(ctx, queryMember, fund.ID, fund.AuthorID)
	if err != nil {
		return nil, err
	}
	err = tx.Commit(ctx)
	if err != nil {
		return nil, err
	}
	return fund, nil
}

func (r *FundRepository) DeleteFund(ctx context.Context, fundID int) error {
	query := `DELETE FROM app.funds WHERE id = $1`
	_, err := r.DB.Exec(ctx, query, fundID)
	return err
}

func (r *FundRepository) GetInfo(ctx context.Context, reqFund *domain.Fund) (*domain.Fund, error) {
	if reqFund == nil {
		return nil, fmt.Errorf("%w: reqFund is nil", domain.ErrInvalidInput)
	}
	var fund domain.Fund
	query := `
		SELECT id, name, author_id, invite_code, created_at 
		FROM app.funds 
		WHERE id = $1 OR (invite_code = $2 AND $2 <> '') 
		LIMIT 1`

	err := r.DB.QueryRow(ctx, query, reqFund.ID, reqFund.InviteCode).Scan(
		&fund.ID, &fund.Name, &fund.AuthorID, &fund.InviteCode, &fund.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("%w: %w", domain.ErrNotFound, err)
		}
		return nil, err
	}
	return &fund, nil
}

func (r *FundRepository) GetByUserID(ctx context.Context, userID int64, limit int, offset int) ([]domain.Fund, error) {
	query := `
        SELECT f.id, f.name, f.author_id, f.invite_code, f.created_at
        FROM app.funds f
        JOIN app.fund_members fm ON f.id = fm.fund_id
        WHERE fm.user_id = $1
        ORDER BY f.created_at DESC
        LIMIT $2 OFFSET $3`

	allFunds, err := r.DB.Query(ctx, query, userID, limit, offset)
	if err != nil {
		slog.Debug(err.Error())
		return nil, err
	}
	defer allFunds.Close()

	funds := make([]domain.Fund, 0)
	for allFunds.Next() {
		var fund domain.Fund
		err = allFunds.Scan(&fund.ID, &fund.Name, &fund.AuthorID, &fund.InviteCode, &fund.CreatedAt)
		if err != nil {
			return nil, err
		}
		funds = append(funds, fund)
	}
	if err := allFunds.Err(); err != nil {
		return nil, err
	}
	return funds, nil
}

func (r *FundRepository) AddMember(ctx context.Context, fundID int, userID int64) error {
	queryMember := `INSERT INTO app.fund_members (fund_id, user_id) VALUES ($1, $2)`
	_, err := r.DB.Exec(ctx, queryMember, fundID, userID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return fmt.Errorf("%w: %w", domain.ErrAlreadyMember, err)
		}
		return err
	}
	return nil
}

func (r *FundRepository) IsMember(ctx context.Context, fundID int, userID int64) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM app.fund_members WHERE user_id = $1 AND fund_id = $2)`

	var exists bool
	err := r.DB.QueryRow(ctx, query, userID, fundID).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (r *FundRepository) GetMembers(ctx context.Context, fundID int) ([]domain.User, error) {
	users := make([]domain.User, 0)

	query := `SELECT f.user_id, u.tg_id, u.username, u.first_name, u.is_virtual, u.created_at
				FROM app.fund_members f
				JOIN app.users u ON f.user_id = u.id
				WHERE f.fund_id = $1`

	rows, err := r.DB.Query(ctx, query, fundID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var user domain.User
		var username sql.NullString
		var isVirtual sql.NullBool
		err = rows.Scan(&user.ID, &user.TgID, &username, &user.FirstName, &isVirtual, &user.CreatedAt)
		if err != nil {
			return nil, err
		}
		user.Username = username.String
		user.IsVirtual = isVirtual.Bool
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return users, nil
}

func (r *FundRepository) GetVirtualUsers(ctx context.Context, fundID int, offset, limit int) ([]domain.User, error) {
	users := make([]domain.User, 0)
	query := `SELECT fm.user_id, u.first_name
FROM app.fund_members fm 
JOIN app.users u ON fm.user_id = u.id
WHERE fm.fund_id = $1 AND u.is_virtual = true
LIMIT $2 OFFSET $3`
	rows, err := r.DB.Query(ctx, query, fundID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var user domain.User
		err = rows.Scan(&user.ID, &user.FirstName)
		if err != nil {
			return nil, err
		}
		user.IsVirtual = true
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return users, nil
}

func (r *FundRepository) RemoveUser(ctx context.Context, fundID int, userID int64) error {
	query := `DELETE FROM app.fund_members
WHERE fund_id = $1 AND user_id = $2`
	_, err := r.DB.Exec(ctx, query, fundID, userID)
	return err
}
