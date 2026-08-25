-- Admin credit grants: positive credit entries created directly by an operator,
-- NOT derived from a validated result (credit_ledger rows are strictly
-- result-derived: leaf_id/work_unit_id/result_id are NOT NULL with FKs). Grants
-- live in their own append-only table so the result-ledger's shape, its unique
-- per-result constraint, and every consumer query stay untouched.
--
-- Balance surfaces that must include grants are updated explicitly in code
-- (ComputeVolunteerBreakdown totals + timelines). Reversal of a mistaken grant is
-- out of scope for this migration: corrections are recorded as follow-up work.

CREATE TABLE public.credit_grants (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    volunteer_id uuid NOT NULL,
    credit_amount numeric(18,6) NOT NULL,
    reason character varying(64) NOT NULL,
    note text,
    created_by character varying(30) NOT NULL DEFAULT 'OPERATOR',
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT credit_grants_pkey PRIMARY KEY (id),
    CONSTRAINT credit_grants_amount_check CHECK ((credit_amount > (0)::numeric)),
    CONSTRAINT credit_grants_reason_check CHECK (reason ~ '^[A-Z0-9_]{1,64}$'),
    CONSTRAINT credit_grants_volunteer_id_fkey FOREIGN KEY (volunteer_id)
        REFERENCES public.volunteers(id) ON DELETE RESTRICT
);

CREATE INDEX idx_credit_grants_volunteer ON public.credit_grants (volunteer_id, created_at);
