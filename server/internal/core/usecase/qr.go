package usecase

import (
	"context"
	"errors"
	"time"

	"agent-events/server/internal/core/domain"
	"agent-events/server/internal/core/port"
	"agent-events/server/pkg/apperr"
)

const (
	ActionGenerateQR = "qr.generate"
	ActionConsumeQR  = "qr.consume"

	tokenPrefixQR = "aeq_"

	ScanStatusPending   = "pending"
	ScanStatusConnected = "connected"
)

const invalidQRPayloadMessage = "invalid or expired QR code"

type QRConfig struct {
	TokenTTL time.Duration
}

type QRService struct {
	events      port.EventRepository
	attendees   port.AttendeeRepository
	tokens      port.QRTokenRepository
	scans       port.ScanRepository
	connections *ConnectionService
	limiter     port.RateLimiter
	logger      port.Logger
	config      QRConfig
}

func NewQRService(
	events port.EventRepository,
	attendees port.AttendeeRepository,
	tokens port.QRTokenRepository,
	scans port.ScanRepository,
	connections *ConnectionService,
	limiter port.RateLimiter,
	logger port.Logger,
	config QRConfig,
) *QRService {
	return &QRService{
		events:      events,
		attendees:   attendees,
		tokens:      tokens,
		scans:       scans,
		connections: connections,
		limiter:     limiter,
		logger:      logger,
		config:      config,
	}
}

type QRCode struct {
	Payload   string
	ExpiresAt time.Time
}

type ScanResult struct {
	Status     string
	Connection domain.Connection
}

func (s *QRService) Generate(ctx context.Context, userID, eventID string) (QRCode, error) {
	event, err := s.loadScannableEvent(ctx, eventID)
	if err != nil {
		return QRCode{}, err
	}

	now := time.Now().UTC()

	if !sameUTCDay(now, event.Date) {
		return QRCode{}, apperr.Invalid("QR codes are only available on the event day")
	}

	allowed, err := s.limiter.Allow(ctx, "user:"+userID, ActionGenerateQR)
	if err != nil {
		s.logger.Error("qr generate rate limit check failed", port.Err(err))
		return QRCode{}, apperr.Wrap(err, "could not check rate limit")
	}

	if !allowed {
		s.logger.Warn("qr generate rate limited", port.Str("user_id", userID))
		return QRCode{}, apperr.TooMany("QR generation limit reached, try again later")
	}

	isAttendee, err := s.attendees.IsAttendee(ctx, eventID, userID)
	if err != nil {
		s.logger.Error("failed to check attendance for qr generation", port.Err(err), port.Str("event_id", eventID))
		return QRCode{}, apperr.Wrap(err, "could not generate QR code")
	}

	if !isAttendee {
		return QRCode{}, apperr.Forbidden("only attendees can generate QR codes")
	}

	rawToken, hash, err := newToken(tokenPrefixQR)
	if err != nil {
		s.logger.Error("failed to generate qr token", port.Err(err))
		return QRCode{}, apperr.Wrap(err, "could not generate QR code")
	}

	expiresAt := now.Add(s.config.TokenTTL)

	if err := s.tokens.Save(ctx, domain.QRToken{
		TokenHash: hash,
		EventID:   eventID,
		UserID:    userID,
		ExpiresAt: expiresAt,
		CreatedAt: now,
	}); err != nil {
		s.logger.Error("failed to store qr token", port.Err(err), port.Str("event_id", eventID))
		return QRCode{}, apperr.Wrap(err, "could not generate QR code")
	}

	s.logger.Info("qr code generated", port.Str("event_id", eventID), port.Str("user_id", userID))

	return QRCode{Payload: rawToken, ExpiresAt: expiresAt}, nil
}

func (s *QRService) Consume(ctx context.Context, scannerID, eventID, payload string) (ScanResult, error) {
	allowed, err := s.limiter.Allow(ctx, "user:"+scannerID, ActionConsumeQR)
	if err != nil {
		s.logger.Error("qr consume rate limit check failed", port.Err(err))
		return ScanResult{}, apperr.Wrap(err, "could not check rate limit")
	}

	if !allowed {
		s.logger.Warn("qr consume rate limited", port.Str("user_id", scannerID))
		return ScanResult{}, apperr.TooMany("QR scan limit reached, try again later")
	}

	event, err := s.loadScannableEvent(ctx, eventID)
	if err != nil {
		return ScanResult{}, err
	}

	now := time.Now().UTC()

	if !sameUTCDay(now, event.Date) {
		return ScanResult{}, apperr.Invalid("QR codes are only available on the event day")
	}

	isAttendee, err := s.attendees.IsAttendee(ctx, eventID, scannerID)
	if err != nil {
		s.logger.Error("failed to check attendance for qr scan", port.Err(err), port.Str("event_id", eventID))
		return ScanResult{}, apperr.Wrap(err, "could not scan QR code")
	}

	if !isAttendee {
		return ScanResult{}, apperr.Forbidden("only attendees can scan QR codes")
	}

	token, err := s.tokens.GetByHash(ctx, HashToken(payload))
	switch {
	case errors.Is(err, domain.ErrQRTokenNotFound):
		return ScanResult{}, apperr.Invalid(invalidQRPayloadMessage)
	case err != nil:
		s.logger.Error("failed to fetch qr token", port.Err(err))
		return ScanResult{}, apperr.Wrap(err, "could not scan QR code")
	}

	if token.Expired(now) {
		return ScanResult{}, apperr.Invalid("QR code has expired, ask for a fresh one")
	}

	if token.Used() {
		return ScanResult{}, apperr.Invalid("QR code has already been used")
	}

	if token.EventID != eventID {
		return ScanResult{}, apperr.Invalid("QR code does not belong to this event")
	}

	if token.UserID == scannerID {
		return ScanResult{}, apperr.Invalid("you cannot scan your own QR code")
	}

	marked, err := s.tokens.MarkUsed(ctx, token.TokenHash, now)
	if err != nil {
		s.logger.Error("failed to mark qr token used", port.Err(err))
		return ScanResult{}, apperr.Wrap(err, "could not scan QR code")
	}

	if !marked {
		return ScanResult{}, apperr.Invalid("QR code has already been used")
	}

	if err := s.scans.Save(ctx, eventID, scannerID, token.UserID, now); err != nil {
		s.logger.Error("failed to record scan", port.Err(err), port.Str("event_id", eventID))
		return ScanResult{}, apperr.Wrap(err, "could not scan QR code")
	}

	reciprocal, err := s.scans.Exists(ctx, eventID, token.UserID, scannerID)
	if err != nil {
		s.logger.Error("failed to check reciprocal scan", port.Err(err), port.Str("event_id", eventID))
		return ScanResult{}, apperr.Wrap(err, "could not scan QR code")
	}

	if !reciprocal {
		s.logger.Info("qr scan recorded, awaiting reciprocal scan",
			port.Str("event_id", eventID),
			port.Str("scanner_id", scannerID),
			port.Str("scanned_id", token.UserID),
		)
		return ScanResult{Status: ScanStatusPending}, nil
	}

	connection, err := s.connections.ConnectPair(ctx, eventID, scannerID, token.UserID, domain.ConnectionMethodQR)
	if err != nil {
		return ScanResult{}, err
	}

	s.logger.Info("mutual qr scan completed, users connected",
		port.Str("event_id", eventID),
		port.Str("connection_id", connection.ID),
	)

	return ScanResult{Status: ScanStatusConnected, Connection: connection}, nil
}

func (s *QRService) loadScannableEvent(ctx context.Context, eventID string) (domain.Event, error) {
	event, err := s.events.Get(ctx, eventID)
	switch {
	case errors.Is(err, domain.ErrEventNotFound):
		return domain.Event{}, apperr.NotFound("event not found")
	case err != nil:
		s.logger.Error("failed to fetch event for qr", port.Err(err), port.Str("event_id", eventID))
		return domain.Event{}, apperr.Wrap(err, "could not load event")
	}

	if !event.Onsite() {
		return domain.Event{}, apperr.Invalid("online events do not use QR codes")
	}

	return event, nil
}

func sameUTCDay(a, b time.Time) bool {
	ay, am, ad := a.UTC().Date()
	by, bm, bd := b.UTC().Date()

	return ay == by && am == bm && ad == bd
}
