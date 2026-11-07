package handler

import (
	"net/http"
	"strconv"
	"time"

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
		admin.GET("/security/events", h.securityEvents)
		admin.GET("/security/stats", h.securityStats)
		admin.GET("/sessions", h.adminSessions)
		admin.GET("/users/:id/sessions", h.userSessions)
		admin.GET("/users/:id/devices", h.userDevices)
		admin.GET("/users/:id/events", h.userEvents)
		admin.POST("/users/:id/block", h.blockUser)
		admin.POST("/users/:id/unblock", h.unblockUser)
		admin.POST("/users/:id/logout-all", h.logoutUserSessions)
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

// --- security / audit ---

func parseTimeQuery(c *gin.Context, key string) *time.Time {
	v := c.Query(key)
	if v == "" {
		return nil
	}
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return &t
	}
	if t, err := time.Parse("2006-01-02", v); err == nil {
		return &t
	}
	return nil
}

func (h *AdminHandler) securityEvents(c *gin.Context) {
	filter := &model.AuthEventFilter{
		Email: c.Query("email"),
		Event: c.Query("event"),
		IP:    c.Query("ip"),
		From:  parseTimeQuery(c, "from"),
		To:    parseTimeQuery(c, "to"),
	}
	if v := c.Query("user_id"); v != "" {
		if id, err := uuid.Parse(v); err == nil {
			filter.UserID = &id
		}
	}
	if v := c.Query("success"); v == "true" || v == "false" {
		b := v == "true"
		filter.Success = &b
	}
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			filter.Limit = n
		}
	}
	if v := c.Query("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			filter.Offset = n
		}
	}
	events, total, err := h.repo.ListAuthEvents(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: events, Meta: gin.H{
		"total": total, "limit": filter.Limit, "offset": filter.Offset,
	}})
}

func (h *AdminHandler) securityStats(c *gin.Context) {
	hours := 24
	if v := c.Query("hours"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 24*90 {
			hours = n
		}
	}
	since := time.Now().Add(-time.Duration(hours) * time.Hour)
	stats, err := h.repo.AdminSecurityStats(c.Request.Context(), since)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: gin.H{
		"window_hours": hours,
		"since":        since,
		"stats":        stats,
	}})
}

func (h *AdminHandler) adminSessions(c *gin.Context) {
	limit, offset := 100, 0
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}
	if v := c.Query("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	items, total, err := h.repo.ListAllSessions(c.Request.Context(), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: items, Meta: gin.H{
		"total": total, "limit": limit, "offset": offset,
	}})
}

func (h *AdminHandler) userSessions(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: "invalid user id"})
		return
	}
	items, err := h.repo.ListSessions(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: gin.H{"sessions": items, "total": len(items)}})
}

func (h *AdminHandler) userDevices(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: "invalid user id"})
		return
	}
	items, err := h.repo.ListUserDevices(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: gin.H{"devices": items, "total": len(items)}})
}

func (h *AdminHandler) userEvents(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: "invalid user id"})
		return
	}
	limit, offset := 50, 0
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}
	if v := c.Query("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	events, total, err := h.repo.ListAuthEvents(c.Request.Context(), &model.AuthEventFilter{
		UserID: &userID, Limit: limit, Offset: offset,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: events, Meta: gin.H{
		"total": total, "limit": limit, "offset": offset,
	}})
}

func (h *AdminHandler) blockUser(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: "invalid user id"})
		return
	}
	if uid, ok := currentUserID(c); ok && uid == userID {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: "you cannot block your own account"})
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&req)
	reason := req.Reason
	if reason == "" {
		reason = "blocked by admin"
	}
	u, err := h.repo.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, model.Response{Success: false, Error: "user not found"})
		return
	}
	if err := h.repo.SetUserBlocked(c.Request.Context(), userID, true, reason); err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	revoked, _ := h.repo.RevokeAllSessions(c.Request.Context(), userID, nil)
	h.adminAudit(c, u, "admin_user_blocked", reason)
	c.JSON(http.StatusOK, model.Response{Success: true, Data: gin.H{
		"user_id": userID, "blocked": true, "reason": reason, "revoked_sessions": revoked,
	}})
}

func (h *AdminHandler) unblockUser(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: "invalid user id"})
		return
	}
	u, err := h.repo.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, model.Response{Success: false, Error: "user not found"})
		return
	}
	if err := h.repo.SetUserBlocked(c.Request.Context(), userID, false, ""); err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	h.adminAudit(c, u, "admin_user_unblocked", "")
	c.JSON(http.StatusOK, model.Response{Success: true, Data: gin.H{"user_id": userID, "blocked": false}})
}

func (h *AdminHandler) logoutUserSessions(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: "invalid user id"})
		return
	}
	u, err := h.repo.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, model.Response{Success: false, Error: "user not found"})
		return
	}
	revoked, err := h.repo.RevokeAllSessions(c.Request.Context(), userID, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	h.adminAudit(c, u, "admin_logout_all", "all sessions revoked")
	c.JSON(http.StatusOK, model.Response{Success: true, Data: gin.H{"user_id": userID, "revoked_sessions": revoked}})
}

func (h *AdminHandler) adminAudit(c *gin.Context, u *model.User, event, reason string) {
	e := &model.AuthEvent{
		Event:      event,
		Success:    true,
		Reason:     reason,
		IP:         c.ClientIP(),
		UserAgent:  c.Request.UserAgent(),
		RequestID:  c.GetString("request_id"),
		Metadata:   model.JSONMap{},
		Platform:   "web",
		DeviceName: "admin",
	}
	if u != nil {
		uid := u.ID
		e.UserID = &uid
		e.Email = u.Email
	}
	if uid, ok := currentUserID(c); ok {
		e.Metadata["admin_id"] = uid.String()
	}
	_ = h.repo.CreateAuthEvent(c.Request.Context(), e)
}
