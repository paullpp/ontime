package maps

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"time"
)

// RouteRequest describes the origin/destination for a route lookup.
type RouteRequest struct {
	OriginLat      float64
	OriginLng      float64
	DestinationLat float64
	DestinationLng float64
}

// RoutesClient fetches travel duration from an external routing service.
type RoutesClient interface {
	GetTravelDuration(ctx context.Context, req RouteRequest) (time.Duration, error)
}

// ── Real Google Maps Routes API client ───────────────────────────────────────

const googleRoutesURL = "https://routes.googleapis.com/directions/v2:computeRoutes"

type googleClient struct {
	apiKey     string
	httpClient *http.Client
}

func NewGoogleClient(apiKey string) RoutesClient {
	return &googleClient{
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

type googleRouteRequest struct {
	Origin      googleWaypoint `json:"origin"`
	Destination googleWaypoint `json:"destination"`
	TravelMode  string         `json:"travelMode"`
	RoutingPreference string   `json:"routingPreference"`
	DepartureTime string       `json:"departureTime"`
}

type googleWaypoint struct {
	Location googleLocation `json:"location"`
}

type googleLocation struct {
	LatLng googleLatLng `json:"latLng"`
}

type googleLatLng struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type googleRoutesResponse struct {
	Routes []struct {
		Duration string `json:"duration"` // e.g. "1234s"
	} `json:"routes"`
}

func (c *googleClient) GetTravelDuration(ctx context.Context, req RouteRequest) (time.Duration, error) {
	body := googleRouteRequest{
		Origin: googleWaypoint{Location: googleLocation{
			LatLng: googleLatLng{Latitude: req.OriginLat, Longitude: req.OriginLng},
		}},
		Destination: googleWaypoint{Location: googleLocation{
			LatLng: googleLatLng{Latitude: req.DestinationLat, Longitude: req.DestinationLng},
		}},
		TravelMode:        "DRIVE",
		RoutingPreference: "TRAFFIC_AWARE_OPTIMAL",
		DepartureTime:     time.Now().UTC().Format(time.RFC3339),
	}

	b, err := json.Marshal(body)
	if err != nil {
		return 0, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, googleRoutesURL, bytes.NewReader(b))
	if err != nil {
		return 0, fmt.Errorf("new request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Goog-Api-Key", c.apiKey)
	httpReq.Header.Set("X-Goog-FieldMask", "routes.duration")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return 0, fmt.Errorf("http do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("google routes API returned %d", resp.StatusCode)
	}

	var result googleRoutesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("decode response: %w", err)
	}
	if len(result.Routes) == 0 {
		return 0, fmt.Errorf("no routes returned")
	}

	// Duration format: "1234s"
	var seconds int
	_, err = fmt.Sscanf(result.Routes[0].Duration, "%ds", &seconds)
	if err != nil {
		return 0, fmt.Errorf("parse duration %q: %w", result.Routes[0].Duration, err)
	}
	return time.Duration(seconds) * time.Second, nil
}

// ── Mock client ───────────────────────────────────────────────────────────────

// MockRoutesClient returns a base duration with simulated traffic variation.
// Base is 1200s (20 min). Every call adds ±5 min of random noise.
type MockRoutesClient struct {
	BaseDuration time.Duration
}

func NewMockClient() RoutesClient {
	return &MockRoutesClient{BaseDuration: 20 * time.Minute}
}

func (m *MockRoutesClient) GetTravelDuration(_ context.Context, req RouteRequest) (time.Duration, error) {
	// Simulate traffic: ±5 minutes random variation around base.
	jitterSec := (rand.Intn(600) - 300) // -300..+300 seconds
	d := m.BaseDuration + time.Duration(jitterSec)*time.Second
	if d < time.Minute {
		d = time.Minute
	}
	fmt.Printf("[MockMaps] GetTravelDuration origin=(%.4f,%.4f) dest=(%.4f,%.4f) -> %s\n",
		req.OriginLat, req.OriginLng, req.DestinationLat, req.DestinationLng, d.Round(time.Second))
	return d, nil
}
