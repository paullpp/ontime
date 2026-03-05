package worker

import (
	"context"
	"encoding/json"
	"log"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ontime/server/internal/db"
	"github.com/ontime/server/internal/maps"
	"github.com/ontime/server/internal/models"
	"github.com/ontime/server/internal/notifications"
	"github.com/ontime/server/internal/scheduler"
)

// Processor handles one trip poll cycle.
type Processor struct {
	store     *db.Store
	maps      maps.RoutesClient
	notifier  notifications.Notifier
	scheduler *scheduler.Scheduler
}

func NewProcessor(
	store *db.Store,
	mapsClient maps.RoutesClient,
	notifier notifications.Notifier,
	sched *scheduler.Scheduler,
) *Processor {
	return &Processor{
		store:     store,
		maps:      mapsClient,
		notifier:  notifier,
		scheduler: sched,
	}
}

// Process polls a single trip, updates ETA, and sends notification if needed.
func (p *Processor) Process(ctx context.Context, tripID uuid.UUID) {
	trip, err := p.store.GetTripByID(ctx, tripID)
	if err != nil {
		log.Printf("[worker] get trip %s: %v", tripID, err)
		return
	}

	if trip.Status != models.TripStatusActive {
		// Already notified or cancelled; remove from scheduler.
		_ = p.scheduler.Unschedule(ctx, tripID)
		return
	}

	// Expire trips whose desired arrival has already passed.
	if time.Now().After(trip.DesiredArrivalAt) {
		log.Printf("[worker] trip %s expired (desired arrival at %s passed)", tripID, trip.DesiredArrivalAt)
		p.expireTrip(ctx, trip)
		return
	}

	// Fetch current travel time from Google Maps.
	etaDuration, err := p.maps.GetTravelDuration(ctx, maps.RouteRequest{
		OriginLat:      trip.OriginLat,
		OriginLng:      trip.OriginLng,
		DestinationLat: trip.DestinationLat,
		DestinationLng: trip.DestinationLng,
	})
	if err != nil {
		log.Printf("[worker] maps error for trip %s: %v — will retry", tripID, err)
		// Back off 2 minutes on maps error.
		nextPoll := time.Now().Add(2 * time.Minute)
		_ = p.scheduler.Schedule(ctx, tripID, nextPoll)
		return
	}

	etaSeconds := int(etaDuration.Seconds())
	timeUntilArrival := time.Until(trip.DesiredArrivalAt)
	nextPollAt := nextPollInterval(timeUntilArrival)

	// Hysteresis: ETA is "stable" if within ±3 minutes of previous reading.
	prevETA := time.Duration(trip.PrevETASeconds) * time.Second
	stable := math.Abs(etaDuration.Seconds()-prevETA.Seconds()) <= 180
	stableCount := 0
	if stable && trip.PrevETASeconds > 0 {
		stableCount = trip.StableETACount + 1
	}

	if err := p.store.UpdateTripPollData(ctx, tripID, etaSeconds, stableCount, nextPollAt); err != nil {
		log.Printf("[worker] update poll data for trip %s: %v", tripID, err)
	}
	_ = p.scheduler.Schedule(ctx, tripID, nextPollAt)

	// Send silent ETA update to keep iOS app fresh.
	p.sendSilentUpdate(ctx, trip, etaSeconds)

	// Compute when the user should leave.
	trip.LatestETASeconds = etaSeconds
	shouldLeaveAt := trip.ShouldLeaveAt()
	log.Printf("[worker] trip %s | ETA=%s | leaveAt=%s | stable=%d | timeUntil=%s",
		tripID, etaDuration.Round(time.Second), shouldLeaveAt.Format(time.RFC3339),
		stableCount, timeUntilArrival.Round(time.Second))

	// Notify if:
	//   1. It is time to leave (or past time).
	//   2. ETA has been stable for ≥2 consecutive polls (hysteresis).
	//   3. Notification hasn't been sent yet.
	if time.Now().Before(shouldLeaveAt) {
		return // Not yet time.
	}
	if stableCount < 2 {
		log.Printf("[worker] trip %s: hold notification (stable count %d < 2)", tripID, stableCount)
		return
	}

	p.sendLeaveNow(ctx, trip, etaSeconds)
}

func (p *Processor) sendLeaveNow(ctx context.Context, trip *models.Trip, etaSeconds int) {
	devices, err := p.store.GetActiveDevicesByUserID(ctx, trip.UserID)
	if err != nil || len(devices) == 0 {
		log.Printf("[worker] no active devices for user %s", trip.UserID)
		return
	}

	sentAt := time.Now()
	if err := p.store.MarkTripNotified(ctx, trip.ID, sentAt); err != nil {
		log.Printf("[worker] mark trip notified %s: %v", trip.ID, err)
		return
	}
	_ = p.scheduler.Unschedule(ctx, trip.ID)

	payload, _ := json.Marshal(map[string]any{
		"type":        "leave_now",
		"eta_seconds": etaSeconds,
	})

	for _, device := range devices {
		msgID, err := p.notifier.SendLeaveNow(ctx, device.APNSToken, trip, etaSeconds)
		status := db.NotifLogStatusSent
		if err != nil {
			status = db.NotifLogStatusFailed
			log.Printf("[worker] send leave_now to device %s: %v", device.ID, err)
			// If APNs rejects the token, mark the device inactive.
			if isInvalidToken(err) {
				_ = p.store.MarkDeviceInactive(ctx, device.ID)
			}
		}
		_ = p.store.CreateNotificationLog(ctx, trip.ID, device.ID, msgID, payload, status)
	}
}

func (p *Processor) sendSilentUpdate(ctx context.Context, trip *models.Trip, etaSeconds int) {
	devices, err := p.store.GetActiveDevicesByUserID(ctx, trip.UserID)
	if err != nil {
		return
	}
	for _, device := range devices {
		_, _ = p.notifier.SendSilentETAUpdate(ctx, device.APNSToken, trip, etaSeconds)
	}
}

func (p *Processor) expireTrip(ctx context.Context, trip *models.Trip) {
	_ = p.store.ExpireTrip(ctx, trip.ID)
	_ = p.scheduler.Unschedule(ctx, trip.ID)
}

// nextPollInterval returns the adaptive polling interval based on time remaining.
func nextPollInterval(timeUntilArrival time.Duration) time.Time {
	var interval time.Duration
	switch {
	case timeUntilArrival > 6*time.Hour:
		interval = 30 * time.Minute
	case timeUntilArrival > 2*time.Hour:
		interval = 15 * time.Minute
	case timeUntilArrival > time.Hour:
		interval = 10 * time.Minute
	case timeUntilArrival > 30*time.Minute:
		interval = 5 * time.Minute
	case timeUntilArrival > 15*time.Minute:
		interval = 3 * time.Minute
	default:
		interval = time.Minute
	}
	return time.Now().Add(interval)
}

// isInvalidToken checks if an APNs error indicates an invalid/expired device token.
func isInvalidToken(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "BadDeviceToken") || strings.Contains(msg, "Unregistered")
}
