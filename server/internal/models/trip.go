package models

import (
	"time"

	"github.com/google/uuid"
)

type TripStatus string

const (
	TripStatusActive    TripStatus = "active"
	TripStatusNotified  TripStatus = "notified"
	TripStatusCancelled TripStatus = "cancelled"
	TripStatusExpired   TripStatus = "expired"
)

type Trip struct {
	ID               uuid.UUID  `json:"id"`
	UserID           uuid.UUID  `json:"user_id"`
	OriginLat        float64    `json:"origin_lat"`
	OriginLng        float64    `json:"origin_lng"`
	OriginName       string     `json:"origin_name"`
	DestinationLat   float64    `json:"destination_lat"`
	DestinationLng   float64    `json:"destination_lng"`
	DestinationName  string     `json:"destination_name"`
	DesiredArrivalAt time.Time  `json:"desired_arrival_at"`
	WarningMinutes   int        `json:"warning_minutes"`
	Status           TripStatus `json:"status"`
	// Latest ETA from Google Maps in seconds.
	LatestETASeconds int `json:"latest_eta_seconds"`
	// Previous ETA used for hysteresis check.
	PrevETASeconds int `json:"-"`
	// Consecutive polls where ETA was stable (within ±3 min of prev).
	StableETACount     int        `json:"-"`
	NextPollAt         time.Time  `json:"next_poll_at"`
	NotificationSentAt *time.Time `json:"notification_sent_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// ShouldLeaveAt computes the time the user should depart.
func (t *Trip) ShouldLeaveAt() time.Time {
	eta := time.Duration(t.LatestETASeconds) * time.Second
	warning := time.Duration(t.WarningMinutes) * time.Minute
	return t.DesiredArrivalAt.Add(-eta - warning)
}
