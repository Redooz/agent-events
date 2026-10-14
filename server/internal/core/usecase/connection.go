package usecase

import (
	"context"
	"errors"
	"time"

	"agent-events/server/internal/core/domain"
	"agent-events/server/internal/core/port"
	"agent-events/server/pkg/apperr"
)

const ActionRequestConnection = "connection.request"

type ConnectionService struct {
	events      port.EventRepository
	attendees   port.AttendeeRepository
	requests    port.ConnectionRequestRepository
	connections port.ConnectionRepository
	limiter     port.RateLimiter
	logger      port.Logger
}

func NewConnectionService(
	events port.EventRepository,
	attendees port.AttendeeRepository,
	requests port.ConnectionRequestRepository,
	connections port.ConnectionRepository,
	limiter port.RateLimiter,
	logger port.Logger,
) *ConnectionService {
	return &ConnectionService{
		events:      events,
		attendees:   attendees,
		requests:    requests,
		connections: connections,
		limiter:     limiter,
		logger:      logger,
	}
}

type RequestConnectionResult struct {
	Request    domain.ConnectionRequest
	Connection *domain.Connection
}

func (s *ConnectionService) Request(ctx context.Context, userID, eventID, toUserID string) (RequestConnectionResult, error) {
	allowed, err := s.limiter.Allow(ctx, "user:"+userID, ActionRequestConnection)
	if err != nil {
		s.logger.Error("connection request rate limit check failed", port.Err(err))
		return RequestConnectionResult{}, apperr.Wrap(err, "could not check rate limit")
	}

	if !allowed {
		s.logger.Warn("connection request rate limited", port.Str("user_id", userID))
		return RequestConnectionResult{}, apperr.TooMany("connection request limit reached, try again later")
	}

	event, err := s.events.Get(ctx, eventID)
	switch {
	case errors.Is(err, domain.ErrEventNotFound):
		return RequestConnectionResult{}, apperr.NotFound("event not found")
	case err != nil:
		s.logger.Error("failed to fetch event for connection request", port.Err(err), port.Str("event_id", eventID))
		return RequestConnectionResult{}, apperr.Wrap(err, "could not request connection")
	}

	if !event.Online() {
		return RequestConnectionResult{}, apperr.Invalid("onsite events connect through QR scans")
	}

	if toUserID == userID {
		return RequestConnectionResult{}, apperr.Invalid("cannot connect with yourself")
	}

	self, err := s.attendees.IsAttendee(ctx, eventID, userID)
	if err != nil {
		s.logger.Error("failed to check attendance", port.Err(err), port.Str("event_id", eventID))
		return RequestConnectionResult{}, apperr.Wrap(err, "could not request connection")
	}

	if !self {
		return RequestConnectionResult{}, apperr.Forbidden("you must attend the event to connect")
	}

	other, err := s.attendees.IsAttendee(ctx, eventID, toUserID)
	if err != nil {
		s.logger.Error("failed to check target attendance", port.Err(err), port.Str("event_id", eventID))
		return RequestConnectionResult{}, apperr.Wrap(err, "could not request connection")
	}

	if !other {
		return RequestConnectionResult{}, apperr.Invalid("user is not attending this event")
	}

	_, err = s.connections.Between(ctx, userID, toUserID)
	switch {
	case err == nil:
		return RequestConnectionResult{}, apperr.Invalid("you are already connected")
	case !errors.Is(err, domain.ErrConnectionNotFound):
		s.logger.Error("failed to check existing connection", port.Err(err))
		return RequestConnectionResult{}, apperr.Wrap(err, "could not request connection")
	}

	existing, err := s.requests.FindPair(ctx, eventID, userID, toUserID)
	if err != nil {
		s.logger.Error("failed to look up existing requests", port.Err(err), port.Str("event_id", eventID))
		return RequestConnectionResult{}, apperr.Wrap(err, "could not request connection")
	}

	now := time.Now().UTC()

	for _, request := range existing {
		if request.Status != domain.RequestStatusPending || request.FromUserID != toUserID || request.ToUserID != userID {
			continue
		}

		ok, err := s.requests.MarkAccepted(ctx, request.ID, now)
		if err != nil {
			s.logger.Error("failed to accept reciprocal request", port.Err(err), port.Str("request_id", request.ID))
			return RequestConnectionResult{}, apperr.Wrap(err, "could not request connection")
		}

		if !ok {
			return RequestConnectionResult{}, apperr.Invalid("connection request is no longer pending")
		}

		connection, err := s.ConnectPair(ctx, eventID, userID, toUserID, domain.ConnectionMethodOnline)
		if err != nil {
			return RequestConnectionResult{}, err
		}

		accepted := request
		accepted.Status = domain.RequestStatusAccepted
		accepted.RespondedAt = now

		s.logger.Info("reciprocal connection request auto-accepted",
			port.Str("event_id", eventID),
			port.Str("request_id", request.ID),
		)

		return RequestConnectionResult{Request: accepted, Connection: &connection}, nil
	}

	for _, request := range existing {
		if request.Status == domain.RequestStatusPending && request.FromUserID == userID && request.ToUserID == toUserID {
			return RequestConnectionResult{}, apperr.Invalid("connection request already exists")
		}
	}

	request := domain.ConnectionRequest{
		ID:         newID(),
		EventID:    eventID,
		FromUserID: userID,
		ToUserID:   toUserID,
		Status:     domain.RequestStatusPending,
		CreatedAt:  now,
	}

	if err := s.requests.Save(ctx, request); err != nil {
		if errors.Is(err, domain.ErrRequestExists) {
			return RequestConnectionResult{}, apperr.Invalid("connection request already exists")
		}

		s.logger.Error("failed to store connection request", port.Err(err), port.Str("event_id", eventID))
		return RequestConnectionResult{}, apperr.Wrap(err, "could not request connection")
	}

	s.logger.Info("connection request created",
		port.Str("request_id", request.ID),
		port.Str("event_id", eventID),
		port.Str("from_user_id", userID),
		port.Str("to_user_id", toUserID),
	)

	return RequestConnectionResult{Request: request}, nil
}

func (s *ConnectionService) Accept(ctx context.Context, userID, requestID string) (domain.Connection, error) {
	request, err := s.requests.Get(ctx, requestID)
	switch {
	case errors.Is(err, domain.ErrRequestNotFound):
		return domain.Connection{}, apperr.NotFound("connection request not found")
	case err != nil:
		s.logger.Error("failed to fetch connection request", port.Err(err), port.Str("request_id", requestID))
		return domain.Connection{}, apperr.Wrap(err, "could not accept connection request")
	}

	if request.ToUserID != userID {
		s.logger.Warn("connection request accept denied for non-recipient",
			port.Str("request_id", requestID),
			port.Str("requesting_user_id", userID),
		)
		return domain.Connection{}, apperr.Forbidden("connection request is not yours")
	}

	if request.Status != domain.RequestStatusPending {
		return domain.Connection{}, apperr.Invalid("connection request is no longer pending")
	}

	ok, err := s.requests.MarkAccepted(ctx, requestID, time.Now().UTC())
	if err != nil {
		s.logger.Error("failed to mark request accepted", port.Err(err), port.Str("request_id", requestID))
		return domain.Connection{}, apperr.Wrap(err, "could not accept connection request")
	}

	if !ok {
		return domain.Connection{}, apperr.Invalid("connection request is no longer pending")
	}

	connection, err := s.ConnectPair(ctx, request.EventID, request.FromUserID, request.ToUserID, domain.ConnectionMethodOnline)
	if err != nil {
		return domain.Connection{}, err
	}

	s.logger.Info("connection request accepted",
		port.Str("request_id", requestID),
		port.Str("connection_id", connection.ID),
	)

	return connection, nil
}

func (s *ConnectionService) Decline(ctx context.Context, userID, requestID string) error {
	request, err := s.requests.Get(ctx, requestID)
	switch {
	case errors.Is(err, domain.ErrRequestNotFound):
		return apperr.NotFound("connection request not found")
	case err != nil:
		s.logger.Error("failed to fetch connection request", port.Err(err), port.Str("request_id", requestID))
		return apperr.Wrap(err, "could not decline connection request")
	}

	if request.ToUserID != userID {
		s.logger.Warn("connection request decline denied for non-recipient",
			port.Str("request_id", requestID),
			port.Str("requesting_user_id", userID),
		)
		return apperr.Forbidden("connection request is not yours")
	}

	if request.Status != domain.RequestStatusPending {
		return apperr.Invalid("connection request is no longer pending")
	}

	ok, err := s.requests.MarkDeclined(ctx, requestID, time.Now().UTC())
	if err != nil {
		s.logger.Error("failed to mark request declined", port.Err(err), port.Str("request_id", requestID))
		return apperr.Wrap(err, "could not decline connection request")
	}

	if !ok {
		return apperr.Invalid("connection request is no longer pending")
	}

	s.logger.Info("connection request declined", port.Str("request_id", requestID))

	return nil
}

func (s *ConnectionService) ConnectPair(ctx context.Context, eventID, userA, userB, method string) (domain.Connection, error) {
	a, b := domain.ConnectionPair(userA, userB)

	connection := domain.Connection{
		ID:        newID(),
		EventID:   eventID,
		UserAID:   a,
		UserBID:   b,
		Method:    method,
		CreatedAt: time.Now().UTC(),
	}

	if err := s.connections.Save(ctx, connection); err != nil {
		if errors.Is(err, domain.ErrAlreadyConnected) {
			existing, getErr := s.connections.Between(ctx, a, b)
			if getErr != nil {
				s.logger.Error("failed to fetch existing connection", port.Err(getErr))
				return domain.Connection{}, apperr.Wrap(getErr, "could not connect")
			}

			return existing, nil
		}

		s.logger.Error("failed to store connection", port.Err(err), port.Str("event_id", eventID))
		return domain.Connection{}, apperr.Wrap(err, "could not connect")
	}

	s.logger.Info("connection created",
		port.Str("connection_id", connection.ID),
		port.Str("event_id", eventID),
		port.Str("method", method),
	)

	return connection, nil
}

func (s *ConnectionService) List(ctx context.Context, userID string) ([]domain.ConnectionView, error) {
	connections, err := s.connections.ListForUser(ctx, userID)
	if err != nil {
		s.logger.Error("failed to list connections", port.Err(err), port.Str("user_id", userID))
		return nil, apperr.Wrap(err, "could not list connections")
	}

	return connections, nil
}

func (s *ConnectionService) ListRequests(ctx context.Context, userID string) ([]domain.ConnectionRequestView, error) {
	requests, err := s.requests.ListIncoming(ctx, userID)
	if err != nil {
		s.logger.Error("failed to list connection requests", port.Err(err), port.Str("user_id", userID))
		return nil, apperr.Wrap(err, "could not list connection requests")
	}

	return requests, nil
}
