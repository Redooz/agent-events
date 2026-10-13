package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"agent-events/server/internal/adapters/postgres/gen"
	"agent-events/server/internal/core/domain"
)

type ConnectionRequestRepository struct {
	q *gen.Queries
}

func NewConnectionRequestRepository(pool *pgxpool.Pool) *ConnectionRequestRepository {
	return &ConnectionRequestRepository{q: gen.New(pool)}
}

func (r *ConnectionRequestRepository) Save(ctx context.Context, request domain.ConnectionRequest) error {
	err := r.q.InsertConnectionRequest(ctx, gen.InsertConnectionRequestParams{
		ID:         request.ID,
		EventID:    request.EventID,
		FromUserID: request.FromUserID,
		ToUserID:   request.ToUserID,
		Status:     request.Status,
		CreatedAt:  timestamptz(request.CreatedAt),
	})

	return asUniqueViolation(err, domain.ErrRequestExists)
}

func (r *ConnectionRequestRepository) Get(ctx context.Context, id string) (domain.ConnectionRequest, error) {
	row, err := r.q.GetConnectionRequestByID(ctx, id)
	if err != nil {
		return domain.ConnectionRequest{}, asNotFound(err, domain.ErrRequestNotFound)
	}

	return requestFromRow(row), nil
}

func (r *ConnectionRequestRepository) FindPair(ctx context.Context, eventID, userA, userB string) ([]domain.ConnectionRequest, error) {
	rows, err := r.q.FindPairConnectionRequests(ctx, gen.FindPairConnectionRequestsParams{
		EventID:    eventID,
		FromUserID: userA,
		ToUserID:   userB,
	})
	if err != nil {
		return nil, err
	}

	requests := make([]domain.ConnectionRequest, 0, len(rows))
	for _, row := range rows {
		requests = append(requests, requestFromRow(row))
	}

	return requests, nil
}

func (r *ConnectionRequestRepository) ListIncoming(ctx context.Context, userID string) ([]domain.ConnectionRequestView, error) {
	rows, err := r.q.ListIncomingConnectionRequests(ctx, userID)
	if err != nil {
		return nil, err
	}

	views := make([]domain.ConnectionRequestView, 0, len(rows))
	for _, row := range rows {
		views = append(views, domain.ConnectionRequestView{
			ConnectionRequest: domain.ConnectionRequest{
				ID:          row.ID,
				EventID:     row.EventID,
				FromUserID:  row.FromUserID,
				ToUserID:    row.ToUserID,
				Status:      row.Status,
				CreatedAt:   timeFromTimestamptz(row.CreatedAt),
				RespondedAt: timeFromTimestamptz(row.RespondedAt),
			},
			EventName:     row.EventName,
			FromUserEmail: row.FromUserEmail,
		})
	}

	return views, nil
}

func (r *ConnectionRequestRepository) MarkAccepted(ctx context.Context, id string, at time.Time) (bool, error) {
	affected, err := r.q.AcceptConnectionRequest(ctx, gen.AcceptConnectionRequestParams{ID: id, RespondedAt: timestamptz(at)})
	if err != nil {
		return false, err
	}

	return affected > 0, nil
}

func (r *ConnectionRequestRepository) MarkDeclined(ctx context.Context, id string, at time.Time) (bool, error) {
	affected, err := r.q.DeclineConnectionRequest(ctx, gen.DeclineConnectionRequestParams{ID: id, RespondedAt: timestamptz(at)})
	if err != nil {
		return false, err
	}

	return affected > 0, nil
}

func requestFromRow(row gen.ConnectionRequest) domain.ConnectionRequest {
	return domain.ConnectionRequest{
		ID:          row.ID,
		EventID:     row.EventID,
		FromUserID:  row.FromUserID,
		ToUserID:    row.ToUserID,
		Status:      row.Status,
		CreatedAt:   timeFromTimestamptz(row.CreatedAt),
		RespondedAt: timeFromTimestamptz(row.RespondedAt),
	}
}
