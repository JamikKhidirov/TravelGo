package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go-travel/internal/model"
	"go-travel/internal/service"
)

type PaymentHandler struct {
	paymentSvc *service.PaymentService
}

func NewPaymentHandler(paymentSvc *service.PaymentService) *PaymentHandler {
	return &PaymentHandler{paymentSvc: paymentSvc}
}

func (h *PaymentHandler) RegisterRoutes(r *gin.RouterGroup) {
	payments := r.Group("/payments")
	{
		payments.POST("", h.create)
		payments.GET("/:id", h.get)
		payments.GET("/order/:order_id", h.byOrder)
		payments.POST("/:id/confirm", h.confirm)
	}
}

type createPaymentRequest struct {
	OrderID  string  `json:"order_id" binding:"required"`
	Amount   float64 `json:"amount" binding:"gt=0"`
	Currency string  `json:"currency"`
	Provider string  `json:"provider"`
}

func (h *PaymentHandler) create(c *gin.Context) {
	var req createPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: err.Error()})
		return
	}
	orderID, err := uuid.Parse(req.OrderID)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: "invalid order_id"})
		return
	}
	p := &model.Payment{
		OrderID:  orderID,
		Amount:   req.Amount,
		Currency: req.Currency,
		Status:   "pending",
		Provider: req.Provider,
	}
	if err := h.paymentSvc.CreatePayment(c.Request.Context(), p); err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, model.Response{Success: true, Data: p})
}

func (h *PaymentHandler) get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: "invalid id"})
		return
	}
	p, err := h.paymentSvc.GetPayment(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: p})
}

func (h *PaymentHandler) byOrder(c *gin.Context) {
	orderID, err := uuid.Parse(c.Param("order_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: "invalid order_id"})
		return
	}
	payments, err := h.paymentSvc.ListPaymentsByOrder(c.Request.Context(), orderID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: payments})
}

func (h *PaymentHandler) confirm(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: "invalid id"})
		return
	}
	if err := h.paymentSvc.ConfirmPayment(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: gin.H{"message": "payment confirmed"}})
}
