package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	apns2 "github.com/sideshow/apns2"
	"github.com/sideshow/apns2/token"
	"github.com/ontime/server/internal/models"
)

// Notifier sends push notifications to iOS devices via APNs.
type Notifier interface {
	SendLeaveNow(ctx context.Context, deviceToken string, trip *models.Trip, etaSeconds int) (apnsMessageID string, err error)
	SendSilentETAUpdate(ctx context.Context, deviceToken string, trip *models.Trip, etaSeconds int) (apnsMessageID string, err error)
}

// ── APNs payload types ────────────────────────────────────────────────────────

type apsAlert struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

type apsPayload struct {
	Alert            *apsAlert `json:"alert,omitempty"`
	Sound            string    `json:"sound,omitempty"`
	ContentAvailable int       `json:"content-available,omitempty"`
	InterruptionLevel string   `json:"interruption-level,omitempty"`
}

type leaveNowPayload struct {
	APS    apsPayload `json:"aps"`
	TripID string     `json:"trip_id"`
	Type   string     `json:"type"`
}

type etaUpdatePayload struct {
	APS        apsPayload `json:"aps"`
	TripID     string     `json:"trip_id"`
	Type       string     `json:"type"`
	ETASeconds int        `json:"eta_seconds"`
}

// ── Real APNs client ──────────────────────────────────────────────────────────

type APNSNotifier struct {
	client   *apns2.Client
	bundleID string
}

func NewAPNSNotifier(keyID, teamID, keyFile, bundleID string) (*APNSNotifier, error) {
	authKey, err := token.AuthKeyFromFile(keyFile)
	if err != nil {
		return nil, fmt.Errorf("load APNs key: %w", err)
	}
	t := &token.Token{
		AuthKey: authKey,
		KeyID:   keyID,
		TeamID:  teamID,
	}
	client := apns2.NewTokenClient(t).Production()
	return &APNSNotifier{client: client, bundleID: bundleID}, nil
}

func (n *APNSNotifier) SendLeaveNow(_ context.Context, deviceToken string, trip *models.Trip, etaSeconds int) (string, error) {
	eta := time.Duration(etaSeconds) * time.Second
	leaveAt := trip.ShouldLeaveAt()
	body := fmt.Sprintf("Drive to %s: %s. Depart by %s.",
		trip.DestinationName,
		eta.Round(time.Minute).String(),
		leaveAt.Format("3:04 PM"),
	)
	payload := leaveNowPayload{
		APS: apsPayload{
			Alert:            &apsAlert{Title: "Time to leave!", Body: body},
			Sound:            "default",
			InterruptionLevel: "time-sensitive",
		},
		TripID: trip.ID.String(),
		Type:   "leave_now",
	}
	return n.send(deviceToken, payload, apns2.PriorityHigh)
}

func (n *APNSNotifier) SendSilentETAUpdate(_ context.Context, deviceToken string, trip *models.Trip, etaSeconds int) (string, error) {
	payload := etaUpdatePayload{
		APS:        apsPayload{ContentAvailable: 1},
		TripID:     trip.ID.String(),
		Type:       "eta_update",
		ETASeconds: etaSeconds,
	}
	return n.send(deviceToken, payload, apns2.PriorityLow)
}

func (n *APNSNotifier) send(deviceToken string, payload any, priority int) (string, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal payload: %w", err)
	}
	notification := &apns2.Notification{
		DeviceToken: deviceToken,
		Topic:       n.bundleID,
		Payload:     b,
		Priority:    priority,
	}
	res, err := n.client.Push(notification)
	if err != nil {
		return "", fmt.Errorf("apns push: %w", err)
	}
	if !res.Sent() {
		return res.ApnsID, fmt.Errorf("apns rejected: %s (%d)", res.Reason, res.StatusCode)
	}
	return res.ApnsID, nil
}

// ── Mock notifier ─────────────────────────────────────────────────────────────

type MockNotifier struct{}

func NewMockNotifier() Notifier {
	return &MockNotifier{}
}

func (m *MockNotifier) SendLeaveNow(_ context.Context, deviceToken string, trip *models.Trip, etaSeconds int) (string, error) {
	eta := time.Duration(etaSeconds) * time.Second
	fmt.Printf("[MockAPNS] LeaveNow -> device=%s trip=%s dest=%q eta=%s leaveAt=%s\n",
		deviceToken[:clamp(8, len(deviceToken))],
		trip.ID,
		trip.DestinationName,
		eta.Round(time.Minute),
		trip.ShouldLeaveAt().Format(time.RFC3339),
	)
	return "mock-apns-id-" + trip.ID.String(), nil
}

func (m *MockNotifier) SendSilentETAUpdate(_ context.Context, deviceToken string, trip *models.Trip, etaSeconds int) (string, error) {
	fmt.Printf("[MockAPNS] ETAUpdate -> device=%s trip=%s eta=%ds\n",
		deviceToken[:clamp(8, len(deviceToken))],
		trip.ID,
		etaSeconds,
	)
	return "mock-apns-id-" + trip.ID.String(), nil
}

func clamp(a, b int) int {
	if a < b {
		return a
	}
	return b
}
