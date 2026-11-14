package payments

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/shopspring/decimal"
	"prestamos/internal/loans/schedule"
	"prestamos/internal/platform/db"
	"prestamos/internal/platform/ids"
)

var (
	ErrNotFound            = errors.New("préstamo no encontrado")
	ErrLoanNotActive       = errors.New("préstamo no está activo")
	ErrInvalidInput        = errors.New("datos de pago inválidos")
	ErrOverpayment         = errors.New("el pago supera el saldo pendiente")
	ErrIdempotencyConflict = errors.New("clave de idempotencia ya utilizada con otros datos")
	ErrDuplicateReference  = errors.New("referencia de pago duplicada")
)

type Part struct {
	Method, Reference, Bank, Phone string
	Amount                         decimal.Decimal
}
type Allocation struct {
	ScheduleID                   string
	Principal, Interest, LateFee decimal.Decimal
}
type Input struct {
	LoanID, UserID, IdempotencyKey string
	Amount                         decimal.Decimal
	Parts                          []Part
	Distribution                   *Distribution `json:",omitempty"`
}
type Payment struct {
	ID, LoanID  string
	Amount      decimal.Decimal
	CreatedAt   time.Time
	Parts       []Part
	Allocations []Allocation
}
type Repo struct {
	tx  db.TxRunner
	ids ids.Generator
}

func NewRepo(tx db.TxRunner, gen ids.Generator) *Repo { return &Repo{tx: tx, ids: gen} }

func validMoney(d decimal.Decimal) bool {
	return d.IsPositive() && d.Equal(d.Round(4)) && d.LessThan(decimal.RequireFromString("1000000000000000"))
}
func validate(in Input) error {
	if in.Distribution != nil && !in.Distribution.valid(in.Amount) {
		return ErrInvalidInput
	}
	if strings.TrimSpace(in.LoanID) == "" || strings.TrimSpace(in.UserID) == "" || strings.TrimSpace(in.IdempotencyKey) == "" || len(in.IdempotencyKey) > 200 || !validMoney(in.Amount) || len(in.Parts) == 0 {
		return ErrInvalidInput
	}
	sum := decimal.Zero
	for _, p := range in.Parts {
		if !validMoney(p.Amount) || len(p.Reference) > 100 || len(p.Bank) > 100 || len(p.Phone) > 100 {
			return ErrInvalidInput
		}
		switch p.Method {
		case "cash", "yape", "plin", "transfer":
		default:
			return ErrInvalidInput
		}
		sum = sum.Add(p.Amount)
	}
	if !sum.Equal(in.Amount) {
		return ErrInvalidInput
	}
	return nil
}
func hashInput(in Input) string {
	b, _ := json.Marshal(in)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func (r *Repo) Record(ctx context.Context, tenant string, in Input) (out Payment, err error) {
	if e := validate(in); e != nil {
		return out, e
	}
	hash := hashInput(in)
	err = r.tx.InTenant(ctx, tenant, func(tx pgx.Tx) error {
		if _, e := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text,0))`, tenant); e != nil {
			return e
		}
		var existingID, existingHash string
		e := tx.QueryRow(ctx, `SELECT id::text, request_hash FROM payments WHERE idempotency_key=$1`, in.IdempotencyKey).Scan(&existingID, &existingHash)
		if e == nil {
			if existingHash != hash {
				return ErrIdempotencyConflict
			}
			out, e = readPayment(ctx, tx, existingID)
			return e
		}
		if !errors.Is(e, pgx.ErrNoRows) {
			return e
		}
		var status string
		e = tx.QueryRow(ctx, `SELECT status FROM loans WHERE id=$1::uuid FOR UPDATE`, in.LoanID).Scan(&status)
		if errors.Is(e, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if e != nil {
			return e
		}
		if status != "active" {
			return ErrLoanNotActive
		}
		var allocationOrder, timezone string
		e = tx.QueryRow(ctx, `SELECT coalesce((SELECT allocation_order FROM tenant_settings),'late_fee_interest_principal'),coalesce((SELECT timezone FROM tenant_settings),'America/Lima')`).Scan(&allocationOrder, &timezone)
		if e != nil {
			return e
		}
		rows, e := tx.Query(ctx, `SELECT id::text, (principal_due-principal_paid)::text, (interest_due-interest_paid)::text, (late_fee_accrued-late_fee_paid)::text, due_date < (now() AT TIME ZONE $2)::date FROM loan_schedule WHERE loan_id=$1::uuid ORDER BY due_date, installment_no, id FOR UPDATE`, in.LoanID, timezone)
		if e != nil {
			return e
		}
		var dues []schedule.Due
		var scheduleIDs []string
		for rows.Next() {
			var id, p, i, l string
			var overdue bool
			if e = rows.Scan(&id, &p, &i, &l, &overdue); e != nil {
				rows.Close()
				return e
			}
			pd, e := decimal.NewFromString(p)
			if e != nil {
				rows.Close()
				return e
			}
			idc, e := decimal.NewFromString(i)
			if e != nil {
				rows.Close()
				return e
			}
			ld, e := decimal.NewFromString(l)
			if e != nil {
				rows.Close()
				return e
			}
			scheduleIDs = append(scheduleIDs, id)
			dues = append(dues, schedule.Due{Principal: pd, Interest: idc, LateFee: ld, Overdue: overdue})
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return e
		}
		if in.Distribution == nil && allocationOrder == "interest_principal_late_fee" {
			for n := range dues {
				if dues[n].Overdue {
					dues[n].LateFee, dues[n].Interest, dues[n].Principal = dues[n].Interest, dues[n].Principal, dues[n].LateFee
				}
			}
		}
		allocations, e := allocatePayment(in, dues)
		if errors.Is(e, schedule.ErrOverpayment) {
			return ErrOverpayment
		}
		if e != nil {
			return e
		}
		if in.Distribution == nil && allocationOrder == "interest_principal_late_fee" {
			for n := range allocations {
				if dues[n].Overdue {
					allocations[n].Interest, allocations[n].Principal, allocations[n].LateFee = allocations[n].LateFee, allocations[n].Interest, allocations[n].Principal
				}
			}
		}
		id, e := r.ids.New()
		if e != nil {
			return e
		}
		out = Payment{ID: id, LoanID: in.LoanID, Amount: in.Amount, Parts: in.Parts, Allocations: []Allocation{}}
		method := in.Parts[0].Method
		if len(in.Parts) > 1 {
			method = "mixed"
		}
		e = tx.QueryRow(ctx, `INSERT INTO payments (id,tenant_id,loan_id,received_by,amount,method,idempotency_key,request_hash) VALUES($1::uuid,$2::uuid,$3::uuid,$4::uuid,$5::numeric,$6,$7,$8) RETURNING created_at`, id, tenant, in.LoanID, in.UserID, in.Amount.String(), method, in.IdempotencyKey, hash).Scan(&out.CreatedAt)
		if e != nil {
			return e
		}
		for _, p := range in.Parts {
			pid, e := r.ids.New()
			if e != nil {
				return e
			}
			_, e = tx.Exec(ctx, `INSERT INTO payment_parts(id,tenant_id,payment_id,method,amount,reference,bank,phone) VALUES($1::uuid,$2::uuid,$3::uuid,$4,$5::numeric,nullif($6,''),nullif($7,''),nullif($8,''))`, pid, tenant, id, p.Method, p.Amount.String(), p.Reference, p.Bank, p.Phone)
			if e != nil {
				var pe *pgconn.PgError
				if errors.As(e, &pe) && pe.ConstraintName == "payment_parts_reference_uq" {
					return ErrDuplicateReference
				}
				return e
			}
		}
		totals := []decimal.Decimal{decimal.Zero, decimal.Zero, decimal.Zero}
		for n, a := range allocations {
			if a.Principal.IsZero() && a.Interest.IsZero() && a.LateFee.IsZero() {
				continue
			}
			aid, e := r.ids.New()
			if e != nil {
				return e
			}
			_, e = tx.Exec(ctx, `INSERT INTO payment_allocations(id,tenant_id,payment_id,schedule_id,principal,interest,late_fee) VALUES($1::uuid,$2::uuid,$3::uuid,$4::uuid,$5::numeric,$6::numeric,$7::numeric)`, aid, tenant, id, scheduleIDs[n], a.Principal.String(), a.Interest.String(), a.LateFee.String())
			if e != nil {
				return e
			}
			_, e = tx.Exec(ctx, `UPDATE loan_schedule SET principal_paid=principal_paid+$1::numeric,interest_paid=interest_paid+$2::numeric,late_fee_paid=late_fee_paid+$3::numeric WHERE id=$4::uuid`, a.Principal.String(), a.Interest.String(), a.LateFee.String(), scheduleIDs[n])
			if e != nil {
				return e
			}
			out.Allocations = append(out.Allocations, Allocation{scheduleIDs[n], a.Principal, a.Interest, a.LateFee})
			totals[0] = totals[0].Add(a.Principal)
			totals[1] = totals[1].Add(a.Interest)
			totals[2] = totals[2].Add(a.LateFee)
		}
		for n, typ := range []string{"principal_collection", "interest_collection", "late_fee_collection"} {
			if totals[n].IsZero() {
				continue
			}
			mid, e := r.ids.New()
			if e != nil {
				return e
			}
			_, e = tx.Exec(ctx, `INSERT INTO capital_movements(id,tenant_id,type,amount,loan_id,payment_id,created_by) VALUES($1::uuid,$2::uuid,$3,$4::numeric,$5::uuid,$6::uuid,$7::uuid)`, mid, tenant, typ, totals[n].String(), in.LoanID, id, in.UserID)
			if e != nil {
				return e
			}
		}
		_, e = tx.Exec(ctx, `UPDATE loans SET status='paid',updated_at=now() WHERE id=$1::uuid AND NOT EXISTS (SELECT 1 FROM loan_schedule WHERE loan_id=$1::uuid AND (principal_due>principal_paid OR interest_due>interest_paid OR late_fee_accrued>late_fee_paid))`, in.LoanID)
		if e != nil {
			return e
		}
		aid, e := r.ids.New()
		if e != nil {
			return e
		}
		_, e = tx.Exec(ctx, `INSERT INTO audit_log(id,tenant_id,user_id,action,entity,entity_id) VALUES($1::uuid,$2::uuid,$3::uuid,'payment.create','payment',$4::uuid)`, aid, tenant, in.UserID, id)
		return e
	})
	return out, err
}

func readPayment(ctx context.Context, tx pgx.Tx, id string) (out Payment, err error) {
	var amt string
	err = tx.QueryRow(ctx, `SELECT id::text, loan_id::text,amount::text,created_at FROM payments WHERE id=$1::uuid`, id).Scan(&out.ID, &out.LoanID, &amt, &out.CreatedAt)
	if err != nil {
		return
	}
	out.Amount, err = decimal.NewFromString(amt)
	if err != nil {
		return
	}
	out.Parts = []Part{}
	out.Allocations = []Allocation{}
	rows, e := tx.Query(ctx, `SELECT method,amount::text,coalesce(reference,''),coalesce(bank,''),coalesce(phone,'') FROM payment_parts WHERE payment_id=$1::uuid ORDER BY id`, id)
	if e != nil {
		return out, e
	}
	for rows.Next() {
		var p Part
		var a string
		if e = rows.Scan(&p.Method, &a, &p.Reference, &p.Bank, &p.Phone); e != nil {
			rows.Close()
			return out, e
		}
		p.Amount, e = decimal.NewFromString(a)
		if e != nil {
			rows.Close()
			return out, e
		}
		out.Parts = append(out.Parts, p)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	rows, e = tx.Query(ctx, `SELECT schedule_id::text,principal::text,interest::text,late_fee::text FROM payment_allocations WHERE payment_id=$1::uuid ORDER BY id`, id)
	if e != nil {
		return out, e
	}
	for rows.Next() {
		var a Allocation
		var p, i, l string
		if e = rows.Scan(&a.ScheduleID, &p, &i, &l); e != nil {
			rows.Close()
			return out, e
		}
		a.Principal, e = decimal.NewFromString(p)
		if e != nil {
			rows.Close()
			return out, e
		}
		a.Interest, e = decimal.NewFromString(i)
		if e != nil {
			rows.Close()
			return out, e
		}
		a.LateFee, e = decimal.NewFromString(l)
		if e != nil {
			rows.Close()
			return out, e
		}
		out.Allocations = append(out.Allocations, a)
	}
	e = rows.Err()
	rows.Close()
	return out, e
}

func (r *Repo) List(ctx context.Context, tenant, loanID string, limit, offset int, includeTotal bool) (out []Payment, more bool, total *int, err error) {
	out = []Payment{}
	if limit < 1 || limit > 100 || offset < 0 {
		err = ErrInvalidInput
		return
	}
	err = r.tx.InTenant(ctx, tenant, func(tx pgx.Tx) error {
		var exists bool
		if e := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM loans WHERE id=$1::uuid)`, loanID).Scan(&exists); e != nil {
			return e
		}
		if !exists {
			return ErrNotFound
		}
		if includeTotal {
			var n int
			if e := tx.QueryRow(ctx, `SELECT count(*) FROM payments WHERE loan_id=$1::uuid`, loanID).Scan(&n); e != nil {
				return e
			}
			total = &n
		}
		rows, e := tx.Query(ctx, `SELECT id::text FROM payments WHERE loan_id=$1::uuid ORDER BY id DESC LIMIT $2 OFFSET $3`, loanID, limit+1, offset)
		if e != nil {
			return e
		}
		var ids []string
		for rows.Next() {
			var id string
			if e = rows.Scan(&id); e != nil {
				rows.Close()
				return e
			}
			ids = append(ids, id)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return e
		}
		if len(ids) > limit {
			more = true
			ids = ids[:limit]
		}
		for _, id := range ids {
			p, e := readPayment(ctx, tx, id)
			if e != nil {
				return fmt.Errorf("leer pago: %w", e)
			}
			out = append(out, p)
		}
		return nil
	})
	return
}
