package db

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ontime/server/internal/models"
)

type CreateTripParams struct {
	UserID           uuid.UUID
	OriginLat        float64
	OriginLng        float64
	OriginName       string
	DestinationLat   float64
	DestinationLng   float64
	DestinationName  string
	DesiredArrivalAt time.Time
	WarningMinutes   int
	NextPollAt       time.Time
}

type UpdateTripParams struct {
	DesiredArrivalAt time.Time
	WarningMinutes   int
}

type RefreshToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	DeviceID  uuid.UUID
	TokenHash string
	ExpiresAt time.Time
}

type NotificationLogStatus string

const (
	NotifLogStatusSent   NotificationLogStatus = "sent"
	NotifLogStatusFailed NotificationLogStatus = "failed"
)

// Store performs all database operations.
type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// ── Users ─────────────────────────────────────────────────────────────────────

func (s *Store) UpsertUser(ctx context.Context, appleSub, email string) (*models.User, error) {
	const q = `
		INSERT INTO users (apple_sub, email)
		VALUES ($1, $2)
		ON CONFLICT (apple_sub) DO UPDATE SET email = EXCLUDED.email, updated_at = NOW()
		RETURNING id, apple_sub, email, created_at, updated_at`
	row := s.pool.QueryRow(ctx, q, appleSub, email)
	return scanUser(row)
}

func (s *Store) GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	const q = `SELECT id, apple_sub, email, created_at, updated_at FROM users WHERE id = $1`
	row := s.pool.QueryRow(ctx, q, id)
	return scanUser(row)
}

func scanUser(row pgx.Row) (*models.User, error) {
	u := &models.User{}
	err := row.Scan(&u.ID, &u.AppleSub, &u.Email, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("scan user: %w", err)
	}
	return u, nil
}

// ── Devices ───────────────────────────────────────────────────────────────────

func (s *Store) RegisterDevice(ctx context.Context, userID uuid.UUID, apnsToken string) (*models.Device, error) {
	const q = `
		INSERT INTO devices (user_id, apns_token, is_active)
		VALUES ($1, $2, TRUE)
		ON CONFLICT (apns_token) DO UPDATE SET user_id = EXCLUDED.user_id, is_active = TRUE, updated_at = NOW()
		RETURNING id, user_id, apns_token, is_active, created_at, updated_at`
	row := s.pool.QueryRow(ctx, q, userID, apnsToken)
	return scanDevice(row)
}

func (s *Store) DeactivateDevice(ctx context.Context, deviceID, userID uuid.UUID) error {
	const q = `UPDATE devices SET is_active = FALSE, updated_at = NOW() WHERE id = $1 AND user_id = $2`
	_, err := s.pool.Exec(ctx, q, deviceID, userID)
	return err
}

func (s *Store) MarkDeviceInactive(ctx context.Context, deviceID uuid.UUID) error {
	const q = `UPDATE devices SET is_active = FALSE, updated_at = NOW() WHERE id = $1`
	_, err := s.pool.Exec(ctx, q, deviceID)
	return err
}

func (s *Store) GetActiveDevicesByUserID(ctx context.Context, userID uuid.UUID) ([]*models.Device, error) {
	const q = `
		SELECT id, user_id, apns_token, is_active, created_at, updated_at
		FROM devices WHERE user_id = $1 AND is_active = TRUE`
	rows, err := s.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var devices []*models.Device
	for rows.Next() {
		d := &models.Device{}
		if err := rows.Scan(&d.ID, &d.UserID, &d.APNSToken, &d.IsActive, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		devices = append(devices, d)
	}
	return devices, rows.Err()
}

func scanDevice(row pgx.Row) (*models.Device, error) {
	d := &models.Device{}
	err := row.Scan(&d.ID, &d.UserID, &d.APNSToken, &d.IsActive, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("scan device: %w", err)
	}
	return d, nil
}

// ── Refresh Tokens ────────────────────────────────────────────────────────────

func HashToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

func (s *Store) CreateRefreshToken(ctx context.Context, userID, deviceID uuid.UUID, tokenHash string, expiresAt time.Time) error {
	const q = `
		INSERT INTO refresh_tokens (user_id, device_id, token_hash, expires_at)
		VALUES ($1, $2, $3, $4)`
	_, err := s.pool.Exec(ctx, q, userID, deviceID, tokenHash, expiresAt)
	return err
}

func (s *Store) GetRefreshToken(ctx context.Context, tokenHash string) (*RefreshToken, error) {
	const q = `
		SELECT id, user_id, device_id, token_hash, expires_at
		FROM refresh_tokens WHERE token_hash = $1 AND expires_at > NOW()`
	rt := &RefreshToken{}
	err := s.pool.QueryRow(ctx, q, tokenHash).
		Scan(&rt.ID, &rt.UserID, &rt.DeviceID, &rt.TokenHash, &rt.ExpiresAt)
	if err != nil {
		return nil, fmt.Errorf("get refresh token: %w", err)
	}
	return rt, nil
}

func (s *Store) DeleteRefreshToken(ctx context.Context, tokenHash string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM refresh_tokens WHERE token_hash = $1`, tokenHash)
	return err
}

func (s *Store) DeleteAllRefreshTokensByUserID(ctx context.Context, userID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM refresh_tokens WHERE user_id = $1`, userID)
	return err
}

// ── Trips ─────────────────────────────────────────────────────────────────────

func (s *Store) CreateTrip(ctx context.Context, p CreateTripParams) (*models.Trip, error) {
	const q = `
		INSERT INTO trips (
			user_id, origin_lat, origin_lng, origin_name,
			destination_lat, destination_lng, destination_name,
			desired_arrival_at, warning_minutes, next_poll_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		RETURNING id, user_id,
			origin_lat, origin_lng, origin_name,
			destination_lat, destination_lng, destination_name,
			desired_arrival_at, warning_minutes, status,
			latest_eta_seconds, prev_eta_seconds, stable_eta_count,
			next_poll_at, notification_sent_at, created_at, updated_at`
	row := s.pool.QueryRow(ctx, q,
		p.UserID, p.OriginLat, p.OriginLng, p.OriginName,
		p.DestinationLat, p.DestinationLng, p.DestinationName,
		p.DesiredArrivalAt, p.WarningMinutes, p.NextPollAt,
	)
	return scanTrip(row)
}

func (s *Store) GetTripByID(ctx context.Context, tripID uuid.UUID) (*models.Trip, error) {
	const q = tripSelectCols + ` WHERE id = $1`
	return scanTrip(s.pool.QueryRow(ctx, q, tripID))
}

func (s *Store) GetTripByIDAndUserID(ctx context.Context, tripID, userID uuid.UUID) (*models.Trip, error) {
	const q = tripSelectCols + ` WHERE id = $1 AND user_id = $2`
	return scanTrip(s.pool.QueryRow(ctx, q, tripID, userID))
}

func (s *Store) GetActiveTripsByUserID(ctx context.Context, userID uuid.UUID) ([]*models.Trip, error) {
	const q = tripSelectCols + ` WHERE user_id = $1 AND status = 'active' ORDER BY desired_arrival_at`
	rows, err := s.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var trips []*models.Trip
	for rows.Next() {
		t, err := scanTripRow(rows)
		if err != nil {
			return nil, err
		}
		trips = append(trips, t)
	}
	return trips, rows.Err()
}

func (s *Store) UpdateTrip(ctx context.Context, tripID, userID uuid.UUID, p UpdateTripParams) (*models.Trip, error) {
	const q = `
		UPDATE trips SET
			desired_arrival_at = $1, warning_minutes = $2, updated_at = NOW()
		WHERE id = $3 AND user_id = $4
		` + returningTripCols
	return scanTrip(s.pool.QueryRow(ctx, q, p.DesiredArrivalAt, p.WarningMinutes, tripID, userID))
}

func (s *Store) CancelTrip(ctx context.Context, tripID, userID uuid.UUID) error {
	const q = `UPDATE trips SET status='cancelled', updated_at=NOW() WHERE id=$1 AND user_id=$2 AND status='active'`
	tag, err := s.pool.Exec(ctx, q, tripID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *Store) ActivateTrip(ctx context.Context, tripID, userID uuid.UUID) error {
	const q = `
		UPDATE trips SET status='active', notification_sent_at=NULL, updated_at=NOW()
		WHERE id=$1 AND user_id=$2 AND status IN ('notified','cancelled')`
	tag, err := s.pool.Exec(ctx, q, tripID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *Store) UpdateTripPollData(ctx context.Context, tripID uuid.UUID, etaSeconds int, stableCount int, nextPollAt time.Time) error {
	const q = `
		UPDATE trips SET
			prev_eta_seconds = latest_eta_seconds,
			latest_eta_seconds = $1,
			stable_eta_count = $2,
			next_poll_at = $3,
			updated_at = NOW()
		WHERE id = $4`
	_, err := s.pool.Exec(ctx, q, etaSeconds, stableCount, nextPollAt, tripID)
	return err
}

func (s *Store) MarkTripNotified(ctx context.Context, tripID uuid.UUID, sentAt time.Time) error {
	const q = `
		UPDATE trips SET status='notified', notification_sent_at=$1, updated_at=NOW()
		WHERE id=$2 AND notification_sent_at IS NULL`
	tag, err := s.pool.Exec(ctx, q, sentAt, tripID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("trip already notified or not found")
	}
	return nil
}

func (s *Store) ExpireTrip(ctx context.Context, tripID uuid.UUID) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE trips SET status='expired', updated_at=NOW() WHERE id=$1 AND status='active'`,
		tripID)
	return err
}

// GetAllActiveTrips is used on startup to seed the Redis scheduler.
func (s *Store) GetAllActiveTrips(ctx context.Context) ([]*models.Trip, error) {
	const q = tripSelectCols + ` WHERE status = 'active'`
	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var trips []*models.Trip
	for rows.Next() {
		t, err := scanTripRow(rows)
		if err != nil {
			return nil, err
		}
		trips = append(trips, t)
	}
	return trips, rows.Err()
}

// ── Notification Logs ─────────────────────────────────────────────────────────

func (s *Store) CreateNotificationLog(ctx context.Context, tripID, deviceID uuid.UUID, apnsMessageID string, payload []byte, status NotificationLogStatus) error {
	const q = `
		INSERT INTO notification_logs (trip_id, device_id, apns_message_id, payload, status)
		VALUES ($1, $2, $3, $4, $5)`
	_, err := s.pool.Exec(ctx, q, tripID, deviceID, apnsMessageID, payload, string(status))
	return err
}

// ── Helpers ───────────────────────────────────────────────────────────────────

const tripSelectCols = `
	SELECT id, user_id,
		origin_lat, origin_lng, origin_name,
		destination_lat, destination_lng, destination_name,
		desired_arrival_at, warning_minutes, status,
		latest_eta_seconds, prev_eta_seconds, stable_eta_count,
		next_poll_at, notification_sent_at, created_at, updated_at
	FROM trips`

const returningTripCols = `
	RETURNING id, user_id,
		origin_lat, origin_lng, origin_name,
		destination_lat, destination_lng, destination_name,
		desired_arrival_at, warning_minutes, status,
		latest_eta_seconds, prev_eta_seconds, stable_eta_count,
		next_poll_at, notification_sent_at, created_at, updated_at`

func scanTrip(row pgx.Row) (*models.Trip, error) {
	t := &models.Trip{}
	err := row.Scan(
		&t.ID, &t.UserID,
		&t.OriginLat, &t.OriginLng, &t.OriginName,
		&t.DestinationLat, &t.DestinationLng, &t.DestinationName,
		&t.DesiredArrivalAt, &t.WarningMinutes, &t.Status,
		&t.LatestETASeconds, &t.PrevETASeconds, &t.StableETACount,
		&t.NextPollAt, &t.NotificationSentAt, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan trip: %w", err)
	}
	return t, nil
}

func scanTripRow(rows pgx.Rows) (*models.Trip, error) {
	t := &models.Trip{}
	err := rows.Scan(
		&t.ID, &t.UserID,
		&t.OriginLat, &t.OriginLng, &t.OriginName,
		&t.DestinationLat, &t.DestinationLng, &t.DestinationName,
		&t.DesiredArrivalAt, &t.WarningMinutes, &t.Status,
		&t.LatestETASeconds, &t.PrevETASeconds, &t.StableETACount,
		&t.NextPollAt, &t.NotificationSentAt, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan trip row: %w", err)
	}
	return t, nil
}
