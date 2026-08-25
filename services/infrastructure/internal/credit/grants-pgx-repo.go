package credit

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lettuce-compute/infrastructure/internal/apierror"
	"github.com/lettuce-compute/infrastructure/internal/types"
)

const grantColumns = `id, volunteer_id, credit_amount, reason, note, created_by, created_at`

// PgxGrantsRepository is the pgx implementation of GrantsRepository.
type PgxGrantsRepository struct {
	pool *pgxpool.Pool
}

// NewPgxGrantsRepository creates a new PgxGrantsRepository.
func NewPgxGrantsRepository(pool *pgxpool.Pool) *PgxGrantsRepository {
	return &PgxGrantsRepository{pool: pool}
}

// Create appends one grant row. An unknown volunteer_id surfaces the FK violation
// (SQLSTATE 23503) as NotFound; everything else maps to Internal.
func (r *PgxGrantsRepository) Create(ctx context.Context, volunteerID types.ID, amount float64, reason, note, createdBy string) (*Grant, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO credit_grants (volunteer_id, credit_amount, reason, note, created_by)
		VALUES ($1, $2, $3, NULLIF($4, ''), $5)
		RETURNING `+grantColumns,
		volunteerID, amount, reason, note, createdBy,
	)

	var g Grant
	err := row.Scan(&g.ID, &g.VolunteerID, &g.CreditAmount, &g.Reason, &g.Note, &g.CreatedBy, &g.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return nil, apierror.NotFound("volunteer", volunteerID.String())
		}
		return nil, apierror.Internal("failed to create credit grant", err)
	}
	return &g, nil
}

// ListByVolunteer returns a volunteer's grants, newest first.
func (r *PgxGrantsRepository) ListByVolunteer(ctx context.Context, volunteerID types.ID, limit, offset int) ([]*Grant, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+grantColumns+`
		FROM credit_grants
		WHERE volunteer_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT $2 OFFSET $3`,
		volunteerID, limit, offset,
	)
	if err != nil {
		return nil, apierror.Internal("failed to list credit grants", err)
	}
	defer rows.Close()

	grants := []*Grant{}
	for rows.Next() {
		var g Grant
		if scanErr := rows.Scan(&g.ID, &g.VolunteerID, &g.CreditAmount, &g.Reason, &g.Note, &g.CreatedBy, &g.CreatedAt); scanErr != nil {
			return nil, apierror.Internal("failed to scan credit grant", scanErr)
		}
		grants = append(grants, &g)
	}
	if err := rows.Err(); err != nil {
		return nil, apierror.Internal("failed to iterate credit grants", err)
	}
	return grants, nil
}

// SumByVolunteer returns the volunteer's total granted credit (0 when none).
// Used by balance surfaces that must include operator grants.
func (r *PgxGrantsRepository) SumByVolunteer(ctx context.Context, volunteerID types.ID) (float64, error) {
	var total float64
	err := r.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(credit_amount), 0)::float8 FROM credit_grants WHERE volunteer_id = $1`,
		volunteerID,
	).Scan(&total)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil
		}
		return 0, err
	}
	return total, nil
}
