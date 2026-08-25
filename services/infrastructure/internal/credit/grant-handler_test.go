package credit

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lettuce-compute/infrastructure/internal/apierror"
	"github.com/lettuce-compute/infrastructure/internal/types"
)

// --- test doubles -----------------------------------------------------------------------

type grantCall struct {
	volunteerID types.ID
	amount      float64
	reason      string
	note        string
	createdBy   string
}

// fakeGrantsRepo is an in-memory GrantsRepository double recording calls.
type fakeGrantsRepo struct {
	grant       *Grant
	err         error
	list        []*Grant
	createCalls []grantCall
}

func (f *fakeGrantsRepo) Create(_ context.Context, volunteerID types.ID, amount float64, reason, note, createdBy string) (*Grant, error) {
	f.createCalls = append(f.createCalls, grantCall{volunteerID, amount, reason, note, createdBy})
	if f.err != nil {
		return nil, f.err
	}
	return f.grant, nil
}

func (f *fakeGrantsRepo) ListByVolunteer(_ context.Context, _ types.ID, _, _ int) ([]*Grant, error) {
	return f.list, f.err
}

// testAdminHandlerWithGrants wires a handler with a grants repo attached.
func testAdminHandlerWithGrants(g GrantsRepository) *AdminHandler {
	h := testAdminHandler(&fakeAdjRepo{}, &fakeLedgerRepo{})
	return h.WithGrantsRepo(g)
}

// --- grant: authorization ----------------------------------------------------------------

func TestHandleGrant_NonAdminForbidden(t *testing.T) {
	g := &fakeGrantsRepo{}
	h := testAdminHandlerWithGrants(g)

	for _, ctx := range []context.Context{
		context.Background(),
		WithCaller(context.Background(), Caller{IsAdmin: false}),
	} {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/credit/grants",
			strings.NewReader(`{"volunteer_id":"`+types.NewID().String()+`","amount":1,"reason":"X"}`)).
			WithContext(ctx)
		rec := httptest.NewRecorder()
		h.HandleGrant(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("non-admin HandleGrant: status = %d, want 403", rec.Code)
		}
	}
	if len(g.createCalls) != 0 {
		t.Errorf("Create should not be called for non-admin, got %d calls", len(g.createCalls))
	}
}

func TestHandleListGrants_NonAdminForbidden(t *testing.T) {
	h := testAdminHandlerWithGrants(&fakeGrantsRepo{})
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/admin/credit/grants?volunteer_id="+types.NewID().String(), nil) // no caller
	rec := httptest.NewRecorder()
	h.HandleListGrants(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}

// --- grant: fail-closed when not wired ---------------------------------------------------

func TestHandleGrant_NotConfiguredFailsClosed(t *testing.T) {
	h := testAdminHandler(&fakeAdjRepo{}, &fakeLedgerRepo{}) // no WithGrantsRepo
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/credit/grants",
		strings.NewReader(`{"volunteer_id":"`+types.NewID().String()+`","amount":1,"reason":"X"}`)).
		WithContext(adminContext())
	rec := httptest.NewRecorder()
	h.HandleGrant(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("unwired HandleGrant: status = %d, want 500", rec.Code)
	}
}

// --- grant: input validation --------------------------------------------------------------

func TestHandleGrant_Validation(t *testing.T) {
	id := types.NewID().String()
	cases := []struct {
		name string
		body string
	}{
		{"malformed json", `{`},
		{"missing volunteer id", `{"amount":1,"reason":"COORDINATOR_SETTLEMENT"}`},
		{"bad volunteer id", `{"volunteer_id":"not-a-uuid","amount":1,"reason":"X"}`},
		{"missing amount", `{"volunteer_id":"` + id + `","reason":"X"}`},
		{"amount zero", `{"volunteer_id":"` + id + `","amount":0,"reason":"X"}`},
		{"amount negative", `{"volunteer_id":"` + id + `","amount":-2.5,"reason":"X"}`},
		{"missing reason", `{"volunteer_id":"` + id + `","amount":1}`},
		{"reason too long", `{"volunteer_id":"` + id + `","amount":1,"reason":"` + strings.Repeat("a", 65) + `"}`},
		{"lowercase reason", `{"volunteer_id":"` + id + `","amount":1,"reason":"oops"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := &fakeGrantsRepo{}
			h := testAdminHandlerWithGrants(g)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/credit/grants",
				strings.NewReader(tc.body)).WithContext(adminContext())
			rec := httptest.NewRecorder()
			h.HandleGrant(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400 (body %s)", rec.Code, rec.Body.String())
			}
			if len(g.createCalls) != 0 {
				t.Errorf("Create should not be called on invalid input, got %d calls", len(g.createCalls))
			}
		})
	}
}

// --- grant: success + repo error mapping ---------------------------------------------------

func TestHandleGrant_Success(t *testing.T) {
	g := &fakeGrantsRepo{
		grant: &Grant{
			ID:           types.NewID(),
			VolunteerID:  types.NewID(),
			CreditAmount: 12.5,
			Reason:       "COORDINATOR_REQUESTS",
			Note:         "150 requests per coordinator export",
			CreatedBy:    AdjustmentByOperator,
		},
	}
	h := testAdminHandlerWithGrants(g)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/credit/grants",
		strings.NewReader(`{"volunteer_id":"`+g.grant.VolunteerID.String()+`","amount":12.5,`+
			`"reason":"COORDINATOR_REQUESTS","note":"150 requests per coordinator export"}`)).
		WithContext(adminContext())
	rec := httptest.NewRecorder()
	h.HandleGrant(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body %s)", rec.Code, rec.Body.String())
	}
	if len(g.createCalls) != 1 {
		t.Fatalf("Create calls = %d, want 1", len(g.createCalls))
	}
	call := g.createCalls[0]
	if call.amount != 12.5 || call.reason != "COORDINATOR_REQUESTS" || call.createdBy != AdjustmentByOperator {
		t.Errorf("unexpected Create args: %+v", call)
	}
	if !strings.Contains(rec.Body.String(), "COORDINATOR_REQUESTS") {
		t.Errorf("response body missing grant payload: %s", rec.Body.String())
	}
}

func TestHandleGrant_RepoErrorMapped(t *testing.T) {
	g := &fakeGrantsRepo{err: apierror.NotFound("volunteer", types.NewID().String())}
	h := testAdminHandlerWithGrants(g)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/credit/grants",
		strings.NewReader(fmt.Sprintf(`{"volunteer_id":%q,"amount":1,"reason":"X"}`, types.NewID().String()))).
		WithContext(adminContext())
	rec := httptest.NewRecorder()
	h.HandleGrant(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 (body %s)", rec.Code, rec.Body.String())
	}

	g.err = errors.New("db down")
	req = httptest.NewRequest(http.MethodPost, "/api/v1/admin/credit/grants",
		strings.NewReader(fmt.Sprintf(`{"volunteer_id":%q,"amount":1,"reason":"X"}`, types.NewID().String()))).
		WithContext(adminContext())
	rec = httptest.NewRecorder()
	h.HandleGrant(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestHandleListGrants_Passthrough(t *testing.T) {
	g := &fakeGrantsRepo{
		list: []*Grant{{ID: types.NewID(), VolunteerID: types.NewID(), CreditAmount: 5, Reason: "X"}},
	}
	h := testAdminHandlerWithGrants(g)

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/admin/credit/grants?volunteer_id="+types.NewID().String()+"&limit=7&offset=3", nil).
		WithContext(adminContext())
	rec := httptest.NewRecorder()
	h.HandleListGrants(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"data"`) {
		t.Errorf("response body missing data envelope: %s", rec.Body.String())
	}
}
