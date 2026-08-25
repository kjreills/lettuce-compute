package credit

import (
	"context"
	"time"

	"github.com/lettuce-compute/infrastructure/internal/types"
)

// Grant is an operator-created POSITIVE credit entry that is not derived from a
// validated result — e.g. crediting a worker for inference requests the head
// cannot see (the coordinator reports those, the operator settles from its
// numbers). The table is append-only: rows are never updated or deleted, and a
// mistaken grant is corrected by a follow-up decision, not by rewriting history.
type Grant struct {
	ID           types.ID  `json:"id"`
	VolunteerID  types.ID  `json:"volunteer_id"`
	CreditAmount float64   `json:"credit_amount"`
	Reason       string    `json:"reason"`
	Note         string    `json:"note,omitempty"`
	CreatedBy    string    `json:"created_by"`
	CreatedAt    time.Time `json:"created_at"`
}

// GrantsRepository is the data-access interface for admin credit grants.
type GrantsRepository interface {
	// Create appends one grant row and returns it populated with the DB-generated
	// id and timestamps. An unknown volunteer_id surfaces the FK violation as a
	// NotFound apierror; other failures are Internal.
	Create(ctx context.Context, volunteerID types.ID, amount float64, reason, note, createdBy string) (*Grant, error)
	// ListByVolunteer returns a volunteer's grants, newest first.
	ListByVolunteer(ctx context.Context, volunteerID types.ID, limit, offset int) ([]*Grant, error)
}
