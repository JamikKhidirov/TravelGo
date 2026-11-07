package service

import (
	"context"
	"log/slog"
	"time"

	"go-travel/internal/repository"
)

// StartJanitor periodically expires old sessions, OAuth codes, audit events and
// per-session IP rows. It runs until ctx is cancelled.
func StartJanitor(ctx context.Context, repo *repository.TravelRepository, interval, eventRetention time.Duration) {
	if interval <= 0 {
		interval = 30 * time.Minute
	}
	if eventRetention <= 0 {
		eventRetention = 180 * 24 * time.Hour
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	run := func() {
		cleanupCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		res, err := repo.CleanupExpired(cleanupCtx, eventRetention)
		if err != nil {
			slog.Warn("security janitor failed", "err", err)
			return
		}
		slog.Info("security janitor done",
			"revoked_sessions", res.RevokedSessions,
			"deleted_oauth_codes", res.DeletedCodes,
			"deleted_events", res.DeletedEvents,
			"deleted_session_ips", res.DeletedIPs,
		)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}
