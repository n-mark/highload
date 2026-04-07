package store

import (
	"context"
	"sync/atomic"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ReplicaPool round-robins queries across multiple pgx pools (read replicas).
type ReplicaPool struct {
	pools []*pgxpool.Pool
	next  atomic.Uint64
}

func NewReplicaPool(pools []*pgxpool.Pool) *ReplicaPool {
	return &ReplicaPool{pools: pools}
}

func (r *ReplicaPool) pool() *pgxpool.Pool {
	if len(r.pools) == 1 {
		return r.pools[0]
	}
	i := r.next.Add(1) % uint64(len(r.pools))
	return r.pools[i]
}

func (r *ReplicaPool) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return r.pool().Exec(ctx, sql, args...)
}

func (r *ReplicaPool) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return r.pool().Query(ctx, sql, args...)
}

func (r *ReplicaPool) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return r.pool().QueryRow(ctx, sql, args...)
}

func (r *ReplicaPool) Close() {
	for _, p := range r.pools {
		p.Close()
	}
}
