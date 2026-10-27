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
	repo       *repository.TravelRepository
	syncSvc    *service.SyncService
	travelSvc  *service.TravelService
	voucherSvc *service.VoucherService
	ticketSvc  *service.TicketService
}

func NewAdminHandler(repo *repository.TravelRepository, syncSvc *service.SyncService, travelSvc *service.TravelService, voucherSvc *service.VoucherService, ticketSvc *service.TicketService) *AdminHandler {
	return &AdminHandler{repo: repo, syncSvc: syncSvc, travelSvc: travelSvc, voucherSvc: voucherSvc, ticketSvc: ticketSvc}
}

func (h *AdminHandler) RegisterRoutes(r *gin.RouterGroup, guards ...gin.HandlerFunc) {
	admin := r.Group("/admin", guards...)
	{
		admin.GET("/stats", h.stats)
		admin.POST("/sync", h.sync)
		admin.GET("/users", h.listUsers)
		admin.GET("/orders", h.listOrders)
		admin.POST("/orders/:id/status", h.updateOrderStatus)
		admin.POST("/orders/:id/partner", h.assignOrderPartner)
		admin.POST("/destinations", h.createDestination)
		admin.POST("/tours", h.createTour)
		admin.POST("/vouchers", h.createVoucher)
		admin.POST("/tickets", h.createTicket)
		admin.GET("/sync-jobs", h.listSyncJobs)
	}
}

func (h *AdminHandler) stats(c *gin.Context) {
	dests, _ := h.repo.CountDestinations(c.Request.Context())
	tours, _ := h.repo.CountTours(c.Request.Context())
	users, _ := h.repo.CountUsers(c.Request.Context())
	orders, _ := h.repo.CountOrders(c.Request.Context())
	revenue, _ := h.repo.SumRevenue(c.Request.Context())
	tickets, _ := h.repo.CountTickets(c.Request.Context())
	devices, _ := h.repo.CountDevices(c.Request.Context())
	pushes, _ := h.repo.CountPushNotifications(c.Request.Context())
	c.JSON(http.StatusOK, model.Response{Success: true, Data: gin.H{
		"destinations": dests,
		"tours":        tours,
		"users":        users,
		"orders":       orders,
		"tickets":      tickets,
		"devices":      devices,
		"pushes":       pushes,
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
	var d model.Destination
	if err := c.ShouldBindJSON(&d); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: err.Error()})
		return
	}
	if err := h.travelSvc.CreateDestination(c.Request.Context(), &d); err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, model.Response{Success: true, Data: d})
}

func (h *AdminHandler) createTour(c *gin.Context) {
	var t model.Tour
	if err := c.ShouldBindJSON(&t); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: err.Error()})
		return
	}
	if err := h.travelSvc.CreateTour(c.Request.Context(), &t); err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, model.Response{Success: true, Data: t})
}

func (h *AdminHandler) createVoucher(c *gin.Context) {
	var v model.Voucher
	if err := c.ShouldBindJSON(&v); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: err.Error()})
		return
	}
	if err := h.voucherSvc.CreateVoucher(c.Request.Context(), &v); err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, model.Response{Success: true, Data: v})
}

func (h *AdminHandler) assignOrderPartner(c *gin.Context) {
	orderID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: "invalid order id"})
		return
	}
	var req struct {
		PartnerID string `json:"partner_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: err.Error()})
		return
	}
	partnerID, err := uuid.Parse(req.PartnerID)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: "invalid partner_id"})
		return
	}
	partner, err := h.repo.GetUserByID(c.Request.Context(), partnerID)
	if err != nil || partner.Role != "partner" {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: "partner user not found"})
		return
	}
	if err := h.repo.UpdateOrderPartner(c.Request.Context(), orderID, partnerID); err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: gin.H{"order_id": orderID, "partner_id": partnerID}})
}

func (h *AdminHandler) createTicket(c *gin.Context) {
	var t model.Ticket
	if err := c.ShouldBindJSON(&t); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: err.Error()})
		return
	}
	if err := h.ticketSvc.Create(c.Request.Context(), &t); err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, model.Response{Success: true, Data: t})
}

func (h *AdminHandler) listSyncJobs(c *gin.Context) {
	jobs, err := h.repo.GetSyncJobs(c.Request.Context(), 100)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: jobs})
}
