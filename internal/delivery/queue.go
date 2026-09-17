package delivery

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"pulseroute/internal/security"
	"time"
)

type Job struct {
	ID, Token, EventID, ProjectID, EndpointID, URL, Secret string
	Payload                                                []byte
	Attempt, Generation, MaxAttempts, BaseDelay, Rate      int
	Active                                                 bool
}
type Queue struct {
	DB    *pgxpool.Pool
	Lease time.Duration
}

func (q Queue) Claim(ctx context.Context) (Job, error) {
	var j Job
	j.Token = security.Token("lease_")
	e := q.DB.QueryRow(ctx, `WITH candidate AS (SELECT id FROM deliveries WHERE (status IN ('pending','retrying') AND next_attempt_at<=now()) OR (status='processing' AND lease_until<now()) ORDER BY next_attempt_at,id FOR UPDATE SKIP LOCKED LIMIT 1), claimed AS (UPDATE deliveries d SET status='processing',lease_until=now()+$2::interval,lease_token=$1,updated_at=now() FROM candidate c WHERE d.id=c.id RETURNING d.*) SELECT d.id,e.event_id,e.project_id,d.endpoint_id,ep.url,ep.secret_cipher,e.payload,d.attempt_count+1,d.generation,ep.max_attempts,ep.base_delay_seconds,ep.rate_per_second,ep.active AND ep.deleted_at IS NULL FROM claimed d JOIN events e ON e.id=d.event_id JOIN endpoints ep ON ep.id=d.endpoint_id`, j.Token, q.Lease.String()).Scan(&j.ID, &j.EventID, &j.ProjectID, &j.EndpointID, &j.URL, &j.Secret, &j.Payload, &j.Attempt, &j.Generation, &j.MaxAttempts, &j.BaseDelay, &j.Rate, &j.Active)
	return j, e
}

var ErrStale = errors.New("delivery lease no longer owned")

func (q Queue) Defer(ctx context.Context, j Job, status string, delay time.Duration) error {
	tag, e := q.DB.Exec(ctx, "UPDATE deliveries SET status=$3,next_attempt_at=now()+$4::interval,lease_token=NULL,lease_until=NULL,updated_at=now() WHERE id=$1 AND lease_token=$2 AND status='processing'", j.ID, j.Token, status, delay.String())
	if e == nil && tag.RowsAffected() == 0 {
		return ErrStale
	}
	return e
}
func (q Queue) Finish(ctx context.Context, j Job, statusCode int, duration time.Duration, reason string) error {
	tx, e := q.DB.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	var id string
	e = tx.QueryRow(ctx, "SELECT id FROM deliveries WHERE id=$1 AND lease_token=$2 AND status='processing' FOR UPDATE", j.ID, j.Token).Scan(&id)
	if errors.Is(e, pgx.ErrNoRows) {
		return ErrStale
	}
	if e != nil {
		return e
	}
	status := "succeeded"
	delay := time.Duration(0)
	if statusCode < 200 || statusCode >= 300 {
		status = "dead"
		if Retryable(statusCode) && j.Attempt < j.MaxAttempts {
			status = "retrying"
			delay = Backoff(j.Attempt, j.BaseDelay)
		}
	}
	_, e = tx.Exec(ctx, "INSERT INTO delivery_attempts(id,delivery_id,generation,attempt,status_code,duration_ms,error) VALUES($1,$2,$3,$4,$5,$6,$7)", security.Token("att_"), j.ID, j.Generation, j.Attempt, statusCode, duration.Milliseconds(), reason)
	if e != nil {
		return e
	}
	_, e = tx.Exec(ctx, "UPDATE deliveries SET status=$3,attempt_count=$4,next_attempt_at=now()+$5::interval,lease_token=NULL,lease_until=NULL,updated_at=now() WHERE id=$1 AND lease_token=$2", j.ID, j.Token, status, j.Attempt, delay.String())
	if e != nil {
		return e
	}
	if status == "dead" {
		_, e = tx.Exec(ctx, "INSERT INTO dead_letters(delivery_id,reason) VALUES($1,$2) ON CONFLICT(delivery_id) DO UPDATE SET reason=excluded.reason,created_at=now()", j.ID, reason)
		if e != nil {
			return e
		}
	}
	return tx.Commit(ctx)
}
