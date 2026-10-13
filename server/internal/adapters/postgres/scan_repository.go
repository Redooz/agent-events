package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"agent-events/server/internal/adapters/postgres/gen"
)

type ScanRepository struct {
	q *gen.Queries
}

func NewScanRepository(pool *pgxpool.Pool) *ScanRepository {
	return &ScanRepository{q: gen.New(pool)}
}

func (r *ScanRepository) Save(ctx context.Context, eventID, scannerID, scannedID string, at time.Time) error {
	return r.q.InsertScan(ctx, gen.InsertScanParams{
		EventID:   eventID,
		ScannerID: scannerID,
		ScannedID: scannedID,
		CreatedAt: timestamptz(at),
	})
}

func (r *ScanRepository) Exists(ctx context.Context, eventID, scannerID, scannedID string) (bool, error) {
	return r.q.ScanExists(ctx, gen.ScanExistsParams{
		EventID:   eventID,
		ScannerID: scannerID,
		ScannedID: scannedID,
	})
}
