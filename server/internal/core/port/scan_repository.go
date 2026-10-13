package port

import (
	"context"
	"time"
)

type ScanRepository interface {
	Save(ctx context.Context, eventID, scannerID, scannedID string, at time.Time) error
	Exists(ctx context.Context, eventID, scannerID, scannedID string) (bool, error)
}
