package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"go-travel/internal/middleware"
	"go-travel/internal/model"
	"go-travel/internal/service"
)

type TravelHandler struct {
	svc     *service.TravelService
	syncSvc *service.SyncService
	val     *validator.Validate
}

func NewTravelHandler(svc *service.TravelService, syncSvc *service.SyncService) *TravelHandler {
	return &TravelHandler{svc: svc, syncSvc: syncSvc, val: validator.New()}
}

type paginationResponse struct {
	Page       int `json:"page"`
	PageSize   int `json:"page_size"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

func (h *TravelHandler) RegisterRoutes(r *gin.Engine, mw *middleware.AuthMiddleware) {
	admin := mw.RequireRoles("admin")
	adminOrGuide := mw.RequireRoles("admin", "guide")
	api := r.Group("/api/v1")
	{
		api.GET("/health", h.health)
		api.GET("/destinations", h.listDestinations)
		api.GET("/destinations/search", h.searchDestinations)
		api.GET("/destinations/top", h.topDestinations)
		api.GET("/destinations/countries", h.listCountries)
		api.GET("/destinations/:id", h.getDestination)
		api.POST("/destinations", admin, h.createDestination)
		api.PUT("/destinations/:id", admin, h.updateDestination)
		api.DELETE("/destinations/:id", admin, h.deleteDestination)
		api.GET("/tours", h.listTours)
		api.GET("/tours/search", h.searchTours)
		api.GET("/tours/featured", h.featuredTours)
		api.GET("/tours/:id", h.getTour)
		api.POST("/tours", adminOrGuide, h.createTour)
		api.PUT("/tours/:id", adminOrGuide, h.updateTour)
		api.DELETE("/tours/:id", adminOrGuide, h.deleteTour)
		api.GET("/events", h.listEvents)
		api.POST("/sync", admin, h.sync)
		api.GET("/sync/jobs", admin, h.listSyncJobs)
	}
}

func (h *TravelHandler) health(c *gin.Context) {
	c.JSON(http.StatusOK, model.Response{Success: true, Data: gin.H{"status": "ok"}})
}

func (h *TravelHandler) listDestinations(c *gin.Context) {
	var p model.Pagination
	if err := c.ShouldBindQuery(&p); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: err.Error()})
		return
	}

	destinations, total, err := h.svc.ListDestinations(c.Request.Context(), c.Query("country"), c.Query("city"), &p)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Success: true,
		Data:    destinations,
		Meta:    paginationResponse{Page: p.Page, PageSize: p.PageSize, Total: total, TotalPages: (total + p.PageSize - 1) / p.PageSize},
	})
}

func (h *TravelHandler) getDestination(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: "invalid id"})
		return
	}

	d, err := h.svc.GetDestination(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: d})
}

func (h *TravelHandler) createDestination(c *gin.Context) {
	var d model.Destination
	if err := c.ShouldBindJSON(&d); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: err.Error()})
		return
	}
	if err := h.val.Struct(&d); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: err.Error()})
		return
	}
	if err := h.svc.CreateDestination(c.Request.Context(), &d); err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, model.Response{Success: true, Data: d})
}

func (h *TravelHandler) listTours(c *gin.Context) {
	var p model.Pagination
	if err := c.ShouldBindQuery(&p); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: err.Error()})
		return
	}

	var destID *uuid.UUID
	if did := c.Query("destination_id"); did != "" {
		id, err := uuid.Parse(did)
		if err != nil {
			c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: "invalid destination_id"})
			return
		}
		destID = &id
	}

	var available *bool
	if av := c.Query("available"); av != "" {
		b, err := strconv.ParseBool(av)
		if err != nil {
			c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: "invalid available"})
			return
		}
		available = &b
	}

	var tours []model.Tour
	var total int
	var err error
	if c.GetString("user_role") == "guide" {
		ownerID, parseErr := uuid.Parse(c.GetString("user_id"))
		if parseErr != nil {
			c.JSON(http.StatusUnauthorized, model.Response{Success: false, Error: "invalid guide identity"})
			return
		}
		// ABAC: a guide only sees tours they own.
		tours, total, err = h.svc.ListToursByOwner(c.Request.Context(), ownerID, destID, available, &p)
	} else {
		tours, total, err = h.svc.ListTours(c.Request.Context(), destID, available, &p)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Success: true,
		Data:    tours,
		Meta:    paginationResponse{Page: p.Page, PageSize: p.PageSize, Total: total, TotalPages: (total + p.PageSize - 1) / p.PageSize},
	})
}

func (h *TravelHandler) getTour(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: "invalid id"})
		return
	}

	t, err := h.svc.GetTour(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: t})
}

func (h *TravelHandler) createTour(c *gin.Context) {
	var t model.Tour
	if err := c.ShouldBindJSON(&t); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: err.Error()})
		return
	}
	if err := h.val.Struct(&t); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: err.Error()})
		return
	}
	if c.GetString("user_role") == "guide" {
		ownerID, parseErr := uuid.Parse(c.GetString("user_id"))
		if parseErr != nil {
			c.JSON(http.StatusUnauthorized, model.Response{Success: false, Error: "invalid guide identity"})
			return
		}
		t.OwnerID = ownerID
	}
	if err := h.svc.CreateTour(c.Request.Context(), &t); err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, model.Response{Success: true, Data: t})
}

func (h *TravelHandler) listEvents(c *gin.Context) {
	aggregate := c.Query("aggregate")
	var aggregateID *uuid.UUID
	if aid := c.Query("aggregate_id"); aid != "" {
		id, err := uuid.Parse(aid)
		if err != nil {
			c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: "invalid aggregate_id"})
			return
		}
		aggregateID = &id
	}
	limit := 50
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}

	events, err := h.svc.GetEvents(c.Request.Context(), aggregate, aggregateID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: events})
}

func (h *TravelHandler) sync(c *gin.Context) {
	if h.syncSvc == nil {
		c.JSON(http.StatusNotImplemented, model.Response{Success: false, Error: "sync not configured"})
		return
	}
	if err := h.syncSvc.SyncAll(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: gin.H{"message": "sync completed"}})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
