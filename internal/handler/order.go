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
	orders, total, err := h.orderSvc.ListOrders(c.Request.Context(), userID, &p)
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
	c.JSON(http.StatusOK, model.Response{Success: true, Data: o})
}

func (h *OrderHandler) cancel(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: "invalid id"})
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
	if err := h.orderSvc.UpdateStatus(c.Request.Context(), id, req.Status); err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: gin.H{"message": "status updated"}})
}

var _ = time.Now


