package middleware

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type rateBucket struct {
	tokens float64
	last   time.Time
}

// RateLimiter is a small in-process token bucket keyed by client IP. It is
// intentionally dependency-free; for multi-instance deployments put a shared
// limiter (Redis/gateway) in front of the API.
type RateLimiter struct {
	mu          sync.Mutex
	buckets     map[string]*rateBucket
	ratePerSec  float64
	burst       float64
	lastCleanup time.Time
}

func NewRateLimiter(perMinute int) *RateLimiter {
	if perMinute <= 0 {
		return nil
	}
	burst := float64(perMinute)
	return &RateLimiter{
		buckets:     make(map[string]*rateBucket),
		ratePerSec:  float64(perMinute) / 60.0,
		burst:       burst,
		lastCleanup: time.Now(),
	}
}

func (l *RateLimiter) allow(key string) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()

	if now.Sub(l.lastCleanup) > 10*time.Minute {
		for k, b := range l.buckets {
			if now.Sub(b.last) > 10*time.Minute {
				delete(l.buckets, k)
			}
		}
		l.lastCleanup = now
	}

	b, ok := l.buckets[key]
	if !ok {
		b = &rateBucket{tokens: l.burst, last: now}
		l.buckets[key] = b
	}
	b.tokens += now.Sub(b.last).Seconds() * l.ratePerSec
	if b.tokens > l.burst {
		b.tokens = l.burst
	}
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

func (l *RateLimiter) middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if l == nil {
			c.Next()
			return
		}
		if !l.allow(c.ClientIP()) {
			c.Header("Retry-After", "60")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"success": false,
				"error":   "rate limit exceeded, try again later",
			})
			return
		}
		c.Next()
	}
}

// AdaptiveRateLimit uses a stricter bucket for authentication endpoints and a
// general bucket for everything else.
func AdaptiveRateLimit(global, auth *RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := strings.ToLower(c.Request.URL.Path)
		limiter := global
		if strings.Contains(path, "/auth/") ||
			strings.HasSuffix(path, "/oauth/token") ||
			strings.HasSuffix(path, "/oauth/authorize") ||
			strings.HasSuffix(path, "/auth") {
			limiter = auth
		}
		if limiter == nil || limiter.allow(c.ClientIP()) {
			c.Next()
			return
		}
		c.Header("Retry-After", "60")
		c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
			"success": false,
			"error":   "rate limit exceeded, try again later",
		})
	}
}
