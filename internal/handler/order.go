package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go-travel/internal/model"
	"go-travel/internal/service"
)

type OrderHandler struct {
	orderSvc *service.OrderService
}

func NewOrderHandler(orderSvc *service.OrderService) *OrderHandler {
	return &OrderHandler{orderSvc: orderSvc}
}

func (h *OrderHandler) RegisterRoutes(r *gin.RouterGroup) {
	orders := r.Group("/orders")
	{
		orders.GET("", h.list)
		orders.GET("/:id", h.get)
		orders.POST("/:id/cancel", h.cancel)
		orders.PUT("/:id/status", h.updateStatus)
	}
}

func (h *OrderHandler) list(c *gin.Context) {
	userIDStr := userIDFromContext(c)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: "invalid user_id"})
		return
	}
	var p model.Pagination
	if err := c.ShouldBindQuery(&p); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: err.Error()})
		return
	}
	var orders []model.Order
	var total int
	if c.GetString("user_role") == "partner" {
		// ABAC: a partner only sees orders explicitly assigned to them.
		orders, total, err = h.orderSvc.ListPartnerOrders(c.Request.Context(), userID, &p)
	} else {
		orders, total, err = h.orderSvc.ListOrders(c.Request.Context(), userID, &p)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{
		Success: true,
		Data:    orders,
		Meta:    model.PaginationResponse{Page: p.Page, PageSize: p.PageSize, Total: total, TotalPages: (total + p.PageSize - 1) / p.PageSize},
	})
}

func (h *OrderHandler) get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: "invalid id"})
		return
	}
	o, err := h.orderSvc.GetOrder(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, model.Response{Success: false, Error: err.Error()})
		return
	}
	uid, _ := uuid.Parse(c.GetString("user_id"))
	if !canAccessOrder(c, o, uid) {
		c.JSON(http.StatusForbidden, model.Response{Success: false, Error: "access denied"})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: o})
}

func canAccessOrder(c *gin.Context, o *model.Order, uid uuid.UUID) bool {
	switch c.GetString("user_role") {
	case "admin":
		return true
	case "partner":
		return o.PartnerID == uid
	default:
		return o.UserID == uid
	}
}

func (h *OrderHandler) cancel(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: "invalid id"})
		return
	}
	o, err := h.orderSvc.GetOrder(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, model.Response{Success: false, Error: err.Error()})
		return
	}
	uid, _ := uuid.Parse(c.GetString("user_id"))
	if c.GetString("user_role") != "admin" && o.UserID != uid {
		c.JSON(http.StatusForbidden, model.Response{Success: false, Error: "access denied"})
		return
	}
	if err := h.orderSvc.CancelOrder(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: gin.H{"message": "cancelled"}})
}

func (h *OrderHandler) updateStatus(c *gin.Context) {
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
	o, err := h.orderSvc.GetOrder(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, model.Response{Success: false, Error: err.Error()})
		return
	}
	uid, _ := uuid.Parse(c.GetString("user_id"))
	role := c.GetString("user_role")
	if role == "partner" && o.PartnerID != uid {
		c.JSON(http.StatusForbidden, model.Response{Success: false, Error: "order is assigned to another partner"})
		return
	}
	if role == "user" && o.UserID != uid {
		c.JSON(http.StatusForbidden, model.Response{Success: false, Error: "access denied"})
		return
	}
	if err := h.orderSvc.UpdateStatus(c.Request.Context(), id, req.Status); err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: gin.H{"message": "status updated"}})
}

var _ = time.Now
