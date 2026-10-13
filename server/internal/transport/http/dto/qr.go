package dto

import (
	"strings"
	"time"

	"agent-events/server/internal/core/usecase"
)

type GenerateQRResponse struct {
	Payload   string    `json:"payload"`
	ExpiresAt time.Time `json:"expires_at"`
}

func NewGenerateQRResponse(code usecase.QRCode) GenerateQRResponse {
	return GenerateQRResponse{Payload: code.Payload, ExpiresAt: code.ExpiresAt}
}

type ConsumeQRRequest struct {
	Payload string `json:"payload" validate:"required"`
}

func (r ConsumeQRRequest) ToPayload() string {
	return strings.TrimSpace(r.Payload)
}

type ScanResponse struct {
	Status     string              `json:"status"`
	Connection *ConnectionResponse `json:"connection,omitempty"`
}

func NewScanResponse(result usecase.ScanResult) ScanResponse {
	out := ScanResponse{Status: result.Status}

	if result.Status == usecase.ScanStatusConnected {
		connection := ConnectionResponseFrom(result.Connection)
		out.Connection = &connection
	}

	return out
}
