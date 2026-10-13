package handler_test

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"

	"agent-events/server/internal/core/domain"
	"agent-events/server/internal/core/port"
)

type fakeEventRepo struct {
	mu     sync.RWMutex
	events map[string]domain.Event
}

var _ port.EventRepository = (*fakeEventRepo)(nil)

func newFakeEventRepo() *fakeEventRepo {
	return &fakeEventRepo{events: make(map[string]domain.Event)}
}

func (r *fakeEventRepo) Save(_ context.Context, event domain.Event) (domain.Event, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.events[event.ID] = event

	return event, nil
}

func (r *fakeEventRepo) Get(_ context.Context, id string) (domain.Event, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	event, ok := r.events[id]
	if !ok {
		return domain.Event{}, domain.ErrEventNotFound
	}

	return event, nil
}

func (r *fakeEventRepo) List(_ context.Context) ([]domain.Event, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	events := make([]domain.Event, 0, len(r.events))
	for _, event := range r.events {
		events = append(events, event)
	}

	return events, nil
}

func (r *fakeEventRepo) Update(_ context.Context, event domain.Event) (domain.Event, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.events[event.ID]; !ok {
		return domain.Event{}, domain.ErrEventNotFound
	}

	r.events[event.ID] = event

	return event, nil
}

func (r *fakeEventRepo) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.events[id]; !ok {
		return domain.ErrEventNotFound
	}

	delete(r.events, id)

	return nil
}

type fakeUserRepo struct {
	mu    sync.RWMutex
	users map[string]domain.User
}

var _ port.UserRepository = (*fakeUserRepo)(nil)

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{users: make(map[string]domain.User)}
}

func (r *fakeUserRepo) UpsertByIdentity(_ context.Context, provider, subject, email string) (domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, user := range r.users {
		if user.Provider == provider && user.Subject == subject {
			user.Email = email
			r.users[user.ID] = user

			return user, nil
		}
	}

	user := domain.User{
		ID:        uuid.NewString(),
		Provider:  provider,
		Subject:   subject,
		Email:     email,
		CreatedAt: time.Now().UTC(),
	}
	r.users[user.ID] = user

	return user, nil
}

func (r *fakeUserRepo) Get(_ context.Context, id string) (domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	user, ok := r.users[id]
	if !ok {
		return domain.User{}, domain.ErrUserNotFound
	}

	return user, nil
}

type fakeUserTokenRepo struct {
	mu     sync.RWMutex
	tokens map[string]domain.UserToken
}

var _ port.UserTokenRepository = (*fakeUserTokenRepo)(nil)

func newFakeUserTokenRepo() *fakeUserTokenRepo {
	return &fakeUserTokenRepo{tokens: make(map[string]domain.UserToken)}
}

func (r *fakeUserTokenRepo) Create(_ context.Context, token domain.UserToken) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.tokens[token.TokenHash] = token

	return nil
}

func (r *fakeUserTokenRepo) GetByHash(_ context.Context, tokenHash string) (domain.UserToken, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	token, ok := r.tokens[tokenHash]
	if !ok {
		return domain.UserToken{}, domain.ErrUserTokenNotFound
	}

	return token, nil
}

func (r *fakeUserTokenRepo) DeleteByHash(_ context.Context, tokenHash string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.tokens, tokenHash)

	return nil
}

func (r *fakeUserTokenRepo) DeleteExpired(_ context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	for hash, token := range r.tokens {
		if token.ExpiresAt.Before(now) {
			delete(r.tokens, hash)
		}
	}

	return nil
}

type fakeAgentRepo struct {
	mu     sync.RWMutex
	agents map[string]domain.Agent
}

var _ port.AgentRepository = (*fakeAgentRepo)(nil)

func newFakeAgentRepo() *fakeAgentRepo {
	return &fakeAgentRepo{agents: make(map[string]domain.Agent)}
}

func (r *fakeAgentRepo) CreateIfUnderLimit(_ context.Context, agent domain.Agent, max int) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	count := 0
	for _, existing := range r.agents {
		if existing.UserID == agent.UserID && existing.RevokedAt.IsZero() {
			count++
		}
	}

	if count >= max {
		return false, nil
	}

	r.agents[agent.ID] = agent

	return true, nil
}

func (r *fakeAgentRepo) Get(_ context.Context, id string) (domain.Agent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	agent, ok := r.agents[id]
	if !ok {
		return domain.Agent{}, domain.ErrAgentNotFound
	}

	return agent, nil
}

func (r *fakeAgentRepo) GetByKeyHash(_ context.Context, keyHash string) (domain.Agent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, agent := range r.agents {
		if agent.KeyHash == keyHash {
			return agent, nil
		}
	}

	return domain.Agent{}, domain.ErrAgentNotFound
}

func (r *fakeAgentRepo) ListByUser(_ context.Context, userID string) ([]domain.Agent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	agents := make([]domain.Agent, 0)
	for _, agent := range r.agents {
		if agent.UserID == userID {
			agents = append(agents, agent)
		}
	}

	return agents, nil
}

func (r *fakeAgentRepo) Revoke(_ context.Context, id string, revokedAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	agent, ok := r.agents[id]
	if !ok {
		return domain.ErrAgentNotFound
	}

	if agent.RevokedAt.IsZero() {
		agent.RevokedAt = revokedAt
		r.agents[id] = agent
	}

	return nil
}

func (r *fakeAgentRepo) TouchLastUsed(_ context.Context, id string, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	agent, ok := r.agents[id]
	if !ok {
		return domain.ErrAgentNotFound
	}

	agent.LastUsedAt = at
	r.agents[id] = agent

	return nil
}

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

func (r *fakeQRTokenRepo) DeleteExpired(_ context.Context, _ time.Time) error {
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
