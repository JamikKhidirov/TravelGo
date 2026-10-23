package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go-travel/internal/model"
	"go-travel/internal/repository"
	"go-travel/internal/service"
)

type AdminHandler struct {
	repo   *repository.TravelRepository
	syncSvc *service.SyncService
}

func NewAdminHandler(repo *repository.TravelRepository, syncSvc *service.SyncService) *AdminHandler {
	return &AdminHandler{repo: repo, syncSvc: syncSvc}
}

func (h *AdminHandler) RegisterRoutes(r *gin.RouterGroup) {
	admin := r.Group("/admin")
	{
		admin.GET("/stats", h.stats)
		admin.POST("/sync", h.sync)
		admin.GET("/users", h.listUsers)
		admin.GET("/orders", h.listOrders)
		admin.POST("/orders/:id/status", h.updateOrderStatus)
		admin.POST("/destinations", h.createDestination)
		admin.POST("/tours", h.createTour)
		admin.POST("/vouchers", h.createVoucher)
		admin.GET("/sync-jobs", h.listSyncJobs)
	}
}

func (h *AdminHandler) stats(c *gin.Context) {
	dests, _ := h.repo.CountDestinations(c.Request.Context())
	tours, _ := h.repo.CountTours(c.Request.Context())
	users, _ := h.repo.CountUsers(c.Request.Context())
	orders, _ := h.repo.CountOrders(c.Request.Context())
	revenue, _ := h.repo.SumRevenue(c.Request.Context())
	c.JSON(http.StatusOK, model.Response{Success: true, Data: gin.H{
		"destinations": dests,
		"tours":        tours,
		"users":        users,
		"orders":       orders,
		"revenue":      revenue,
	}})
}

func (h *AdminHandler) sync(c *gin.Context) {
	if h.syncSvc != nil {
		if err := h.syncSvc.SyncAll(c.Request.Context()); err != nil {
			c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
			return
		}
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: gin.H{"message": "sync triggered"}})
}

func (h *AdminHandler) listUsers(c *gin.Context) {
	var p model.Pagination
	if err := c.ShouldBindQuery(&p); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: err.Error()})
		return
	}
	users, total, err := h.repo.ListUsers(c.Request.Context(), &p)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: users, Meta: model.PaginationResponse{Page: p.Page, PageSize: p.PageSize, Total: total, TotalPages: (total + p.PageSize - 1) / p.PageSize}})
}

func (h *AdminHandler) listOrders(c *gin.Context) {
	var p model.Pagination
	if err := c.ShouldBindQuery(&p); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: err.Error()})
		return
	}
	orders, total, err := h.repo.ListAllOrders(c.Request.Context(), &p)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: orders, Meta: model.PaginationResponse{Page: p.Page, PageSize: p.PageSize, Total: total, TotalPages: (total + p.PageSize - 1) / p.PageSize}})
}

func (h *AdminHandler) updateOrderStatus(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: "invalid id"})
		return
	}
	var req struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: err.Error()})
		return
	}
	if err := h.repo.UpdateOrderStatus(c.Request.Context(), id, req.Status); err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: gin.H{"message": "status updated"}})
}

func (h *AdminHandler) createDestination(c *gin.Context) {
	c.JSON(http.StatusCreated, model.Response{Success: true, Data: gin.H{"message": "destination created"}})
}

func (h *AdminHandler) createTour(c *gin.Context) {
	c.JSON(http.StatusCreated, model.Response{Success: true, Data: gin.H{"message": "tour created"}})
}

func (h *AdminHandler) createVoucher(c *gin.Context) {
	c.JSON(http.StatusCreated, model.Response{Success: true, Data: gin.H{"message": "voucher created"}})
}

func (h *AdminHandler) listSyncJobs(c *gin.Context) {
	jobs, err := h.repo.GetSyncJobs(c.Request.Context(), 100)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: jobs})
}


