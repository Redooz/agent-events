package usecase_test

import (
	"context"
	"sync"
	"time"

	"agent-events/server/internal/core/domain"
	"agent-events/server/internal/core/port"
)

type fakeAttendeeRepo struct {
	mu        sync.RWMutex
	attendees map[string]map[string]bool
}

var _ port.AttendeeRepository = (*fakeAttendeeRepo)(nil)

func newFakeAttendeeRepo() *fakeAttendeeRepo {
	return &fakeAttendeeRepo{attendees: make(map[string]map[string]bool)}
}

func (r *fakeAttendeeRepo) Attend(_ context.Context, eventID, userID string, _ time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.attendees[eventID] == nil {
		r.attendees[eventID] = make(map[string]bool)
	}

	r.attendees[eventID][userID] = true

	return nil
}

func (r *fakeAttendeeRepo) IsAttendee(_ context.Context, eventID, userID string) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.attendees[eventID][userID], nil
}

func (r *fakeAttendeeRepo) ListByEvent(_ context.Context, eventID string) ([]domain.Attendee, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var out []domain.Attendee
	for userID := range r.attendees[eventID] {
		out = append(out, domain.Attendee{UserID: userID, Email: userID + "@example.com"})
	}

	return out, nil
}

type fakeConnectionRepo struct {
	mu    sync.RWMutex
	conns map[string]domain.Connection
}

var _ port.ConnectionRepository = (*fakeConnectionRepo)(nil)

func newFakeConnectionRepo() *fakeConnectionRepo {
	return &fakeConnectionRepo{conns: make(map[string]domain.Connection)}
}

func (r *fakeConnectionRepo) Save(_ context.Context, connection domain.Connection) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := connection.UserAID + "|" + connection.UserBID
	if _, ok := r.conns[key]; ok {
		return domain.ErrAlreadyConnected
	}

	r.conns[key] = connection

	return nil
}

func (r *fakeConnectionRepo) Between(_ context.Context, userA, userB string) (domain.Connection, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	a, b := domain.ConnectionPair(userA, userB)

	conn, ok := r.conns[a+"|"+b]
	if !ok {
		return domain.Connection{}, domain.ErrConnectionNotFound
	}

	return conn, nil
}

func (r *fakeConnectionRepo) ListForUser(_ context.Context, userID string) ([]domain.ConnectionView, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var out []domain.ConnectionView
	for _, conn := range r.conns {
		if conn.UserAID != userID && conn.UserBID != userID {
			continue
		}

		other := conn.UserAID
		if other == userID {
			other = conn.UserBID
		}

		out = append(out, domain.ConnectionView{
			Connection:     conn,
			EventName:      "event",
			OtherUserID:    other,
			OtherUserEmail: other + "@example.com",
		})
	}

	return out, nil
}

type fakeRequestRepo struct {
	mu       sync.RWMutex
	requests map[string]domain.ConnectionRequest
}

var _ port.ConnectionRequestRepository = (*fakeRequestRepo)(nil)

func newFakeRequestRepo() *fakeRequestRepo {
	return &fakeRequestRepo{requests: make(map[string]domain.ConnectionRequest)}
}

func (r *fakeRequestRepo) Save(_ context.Context, request domain.ConnectionRequest) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, existing := range r.requests {
		if existing.EventID == request.EventID && existing.FromUserID == request.FromUserID && existing.ToUserID == request.ToUserID {
			return domain.ErrRequestExists
		}
	}

	r.requests[request.ID] = request

	return nil
}

func (r *fakeRequestRepo) Get(_ context.Context, id string) (domain.ConnectionRequest, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	request, ok := r.requests[id]
	if !ok {
		return domain.ConnectionRequest{}, domain.ErrRequestNotFound
	}

	return request, nil
}

func (r *fakeRequestRepo) FindPair(_ context.Context, eventID, userA, userB string) ([]domain.ConnectionRequest, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var out []domain.ConnectionRequest
	for _, request := range r.requests {
		if request.EventID != eventID {
			continue
		}

		forward := request.FromUserID == userA && request.ToUserID == userB
		reverse := request.FromUserID == userB && request.ToUserID == userA

		if forward || reverse {
			out = append(out, request)
		}
	}

	return out, nil
}

func (r *fakeRequestRepo) ListIncoming(_ context.Context, userID string) ([]domain.ConnectionRequestView, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var out []domain.ConnectionRequestView
	for _, request := range r.requests {
		if request.ToUserID != userID || request.Status != domain.RequestStatusPending {
			continue
		}

		out = append(out, domain.ConnectionRequestView{
			ConnectionRequest: request,
			EventName:         "event",
			FromUserEmail:     request.FromUserID + "@example.com",
		})
	}

	return out, nil
}

func (r *fakeRequestRepo) MarkAccepted(_ context.Context, id string, at time.Time) (bool, error) {
	return r.mark(id, domain.RequestStatusAccepted, at)
}

func (r *fakeRequestRepo) MarkDeclined(_ context.Context, id string, at time.Time) (bool, error) {
	return r.mark(id, domain.RequestStatusDeclined, at)
}

func (r *fakeRequestRepo) mark(id, status string, at time.Time) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	request, ok := r.requests[id]
	if !ok || request.Status != domain.RequestStatusPending {
		return false, nil
	}

	request.Status = status
	request.RespondedAt = at
	r.requests[id] = request

	return true, nil
}

type fakeQRTokenRepo struct {
	mu     sync.RWMutex
	tokens map[string]domain.QRToken
}

var _ port.QRTokenRepository = (*fakeQRTokenRepo)(nil)

func newFakeQRTokenRepo() *fakeQRTokenRepo {
	return &fakeQRTokenRepo{tokens: make(map[string]domain.QRToken)}
}

func (r *fakeQRTokenRepo) Save(_ context.Context, token domain.QRToken) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.tokens[token.TokenHash] = token

	return nil
}

func (r *fakeQRTokenRepo) GetByHash(_ context.Context, tokenHash string) (domain.QRToken, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	token, ok := r.tokens[tokenHash]
	if !ok {
		return domain.QRToken{}, domain.ErrQRTokenNotFound
	}

	return token, nil
}

func (r *fakeQRTokenRepo) MarkUsed(_ context.Context, tokenHash string, at time.Time) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	token, ok := r.tokens[tokenHash]
	if !ok || !token.UsedAt.IsZero() {
		return false, nil
	}

	token.UsedAt = at
	r.tokens[tokenHash] = token

	return true, nil
}

func (r *fakeQRTokenRepo) DeleteExpired(_ context.Context, before time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for hash, token := range r.tokens {
		if token.ExpiresAt.Before(before) {
			delete(r.tokens, hash)
		}
	}

	return nil
}

type fakeScanRepo struct {
	mu    sync.RWMutex
	scans map[string]bool
}

var _ port.ScanRepository = (*fakeScanRepo)(nil)

func newFakeScanRepo() *fakeScanRepo {
	return &fakeScanRepo{scans: make(map[string]bool)}
}

func (r *fakeScanRepo) Save(_ context.Context, eventID, scannerID, scannedID string, _ time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.scans[eventID+"|"+scannerID+"|"+scannedID] = true

	return nil
}

func (r *fakeScanRepo) Exists(_ context.Context, eventID, scannerID, scannedID string) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.scans[eventID+"|"+scannerID+"|"+scannedID], nil
}
