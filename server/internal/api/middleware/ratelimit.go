package middleware

import (
	"net/http"
	"sync"

	"github.com/ontime/server/internal/api/respond"
	"golang.org/x/time/rate"
)

// PerIPRateLimiter limits requests per remote IP address.
func PerIPRateLimiter(r rate.Limit, burst int) func(http.Handler) http.Handler {
	type entry struct {
		limiter *rate.Limiter
	}
	var (
		mu      sync.Mutex
		clients = map[string]*entry{}
	)
	getLimiter := func(ip string) *rate.Limiter {
		mu.Lock()
		defer mu.Unlock()
		e, ok := clients[ip]
		if !ok {
			e = &entry{limiter: rate.NewLimiter(r, burst)}
			clients[ip] = e
		}
		return e.limiter
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ip := req.RemoteAddr
			if !getLimiter(ip).Allow() {
				respond.Error(w, http.StatusTooManyRequests, "rate limit exceeded")
				return
			}
			next.ServeHTTP(w, req)
		})
	}
}
