package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"go-travel/internal/adapter/mock"
	"go-travel/internal/adapter/msp2"
	scraper "go-travel/internal/adapter/scraper"
	"go-travel/internal/broker"
	"go-travel/internal/cache"
	"go-travel/internal/config"
	"go-travel/internal/handler"
	"go-travel/internal/repository"
	"go-travel/internal/service"
)

// swaggerSpec — OpenAPI 3.0 спецификация в формате YAML (встроена в бинарник)

// swaggerUI — HTML страница Swagger UI (встроена в бинарник)
const swaggerUI = `<!DOCTYPE html>
<html lang="ru">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>GoTravel API — Swagger UI</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5.9.0/swagger-ui.css" />
  <style>body { margin: 0; padding: 0; } #swagger-ui { max-width: 100%; }</style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5.9.0/swagger-ui-bundle.js" crossorigin></script>
  <script src="https://unpkg.com/swagger-ui-dist@5.9.0/swagger-ui-standalone-preset.js" crossorigin></script>
  <script>
    window.onload = function() {
      SwaggerUIBundle({
        url: "/swagger/doc.yaml",
        dom_id: "#swagger-ui",
        deepLinking: true,
        presets: [
          SwaggerUIBundle.presets.apis,
          SwaggerUIStandalonePreset
        ],
        layout: "StandaloneLayout",
        tryItOutEnabled: true,
        docExpansion: "list"
      });
    };
  </script>
</body>
</html>`

func main() {
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "configs/config.yaml"
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		slog.Error("failed to load config", "err", err)
		os.Exit(1)
	}

	if cfg.App.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	db, err := sqlx.Open("postgres", cfg.DSN())
	if err != nil {
		slog.Error("failed to open db", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		slog.Error("failed to ping db", "err", err)
		os.Exit(1)
	}

	repo := repository.NewTravelRepository(db)
	if err := repo.Migrate(context.Background()); err != nil {
		slog.Error("failed to migrate", "err", err)
		os.Exit(1)
	}
	if err := repo.MigrateExtra(context.Background()); err != nil {
		slog.Error("failed to migrate extra", "err", err)
		os.Exit(1)
	}

	cacheStore := cache.New(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
	defer cacheStore.Close()

	eventBroker := broker.NewMemoryBroker()
	defer eventBroker.Close()

	mockSrc := mock.NewMockSource(cfg.Mock.BaseURL)
	msp2Src := msp2.NewMSP2Source()
	scraperSrc := scraper.NewCityAPISource()

	travelSvc := service.NewTravelService(repo)
	authSvc := service.NewAuthService(repo, "travel-secret")
	cartSvc := service.NewCartService(repo)
	orderSvc := service.NewOrderService(repo)
	paymentSvc := service.NewPaymentService(repo)
	reviewSvc := service.NewReviewService(repo)
	voucherSvc := service.NewVoucherService(repo)
	syncSvc := service.NewSyncService(repo, mockSrc, msp2Src, scraperSrc)

	_ = cacheStore
	_ = eventBroker

	h := handler.NewTravelHandler(travelSvc, syncSvc)
	authH := handler.NewAuthHandler(authSvc)
	cartH := handler.NewCartHandler(cartSvc)
	orderH := handler.NewOrderHandler(orderSvc)
	paymentH := handler.NewPaymentHandler(paymentSvc)
	reviewH := handler.NewReviewHandler(reviewSvc)
	voucherH := handler.NewVoucherHandler(voucherSvc)
	adminH := handler.NewAdminHandler(repo, syncSvc)

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	// Swagger UI — отдаём YAML спецификацию напрямую
	r.GET("/swagger/index.html", func(c *gin.Context) {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, swaggerUI)
	})
	r.GET("/swagger/doc.yaml", func(c *gin.Context) {
		c.Data(http.StatusOK, "application/yaml", []byte(swaggerSpec))
	})

	h.RegisterRoutes(r)

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"success": true, "data": gin.H{"status": "ok"}})
	})

	api := r.Group("/api/v1")
	authH.RegisterRoutes(api)
	cartH.RegisterRoutes(api)
	orderH.RegisterRoutes(api)
	paymentH.RegisterRoutes(api)
	reviewH.RegisterRoutes(api)
	voucherH.RegisterRoutes(api)
	adminH.RegisterRoutes(api)

	wsHub := handler.NewWSHub()
	go wsHub.Run()
	r.GET("/ws", func(c *gin.Context) {
		wsHub.HandleWS(c.Writer, c.Request)
	})
	r.GET("/ws/events", func(c *gin.Context) {
		wsHub.HandleWS(c.Writer, c.Request)
	})

	srv := &http.Server{
		Addr:         ":" + strconv.Itoa(cfg.App.Port),
		Handler:      r,
		ReadTimeout:  cfg.AppTimeout(cfg.App.ReadTimeout),
		WriteTimeout: cfg.AppTimeout(cfg.App.WriteTimeout),
	}

	go func() {
		slog.Info("================================================")
		slog.Info("GoTravel API started")
		slog.Info("REST API:  http://localhost:" + strconv.Itoa(cfg.App.Port))
		slog.Info("Swagger UI: http://localhost:" + strconv.Itoa(cfg.App.Port) + "/swagger/index.html")
		slog.Info("WebSocket: ws://localhost:" + strconv.Itoa(cfg.App.Port) + "/ws/events")
		slog.Info("Health:    http://localhost:" + strconv.Itoa(cfg.App.Port) + "/health")
		slog.Info("================================================")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("server shutdown error", "err", err)
	}

	slog.Info("server stopped")
}

