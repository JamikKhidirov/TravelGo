package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go-travel/internal/model"
	"go-travel/internal/service"
)

type VoucherHandler struct {
	voucherSvc *service.VoucherService
}

func NewVoucherHandler(voucherSvc *service.VoucherService) *VoucherHandler {
	return &VoucherHandler{voucherSvc: voucherSvc}
}

func (h *VoucherHandler) RegisterRoutes(r *gin.RouterGroup) {
	vouchers := r.Group("/vouchers")
	{
		vouchers.GET("", h.list)
		vouchers.GET("/:id", h.get)
		vouchers.POST("", h.create)
		vouchers.PUT("/:id", h.update)
		vouchers.DELETE("/:id", h.delete)
	}
}

type createVoucherRequest struct {
	Title           string  `json:"title" validate:"required"`
	Description     string  `json:"description"`
	DestinationID   string  `json:"destination_id" validate:"required"`
	Price           float64 `json:"price" validate:"gt=0"`
	Currency        string  `json:"currency"`
	DurationNights  int     `json:"duration_nights" validate:"min=1"`
	IncludesFlight  bool    `json:"includes_flight"`
	IncludesHotel   bool    `json:"includes_hotel"`
	IncludesMeal    bool    `json:"includes_meal"`
	ImageURL        string  `json:"image_url"`
	IsActive        bool    `json:"is_active"`
}

func (h *VoucherHandler) list(c *gin.Context) {
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
	vouchers, total, err := h.voucherSvc.ListVouchers(c.Request.Context(), destID, &p)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{
		Success: true,
		Data:    vouchers,
		Meta:    model.PaginationResponse{Page: p.Page, PageSize: p.PageSize, Total: total, TotalPages: (total + p.PageSize - 1) / p.PageSize},
	})
}

func (h *VoucherHandler) get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: "invalid id"})
		return
	}
	v, err := h.voucherSvc.GetVoucher(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: v})
}

func (h *VoucherHandler) create(c *gin.Context) {
	var req createVoucherRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: err.Error()})
		return
	}
	destID, _ := uuid.Parse(req.DestinationID)
	v := &model.Voucher{
		Title:           req.Title,
		Description:     req.Description,
		DestinationID:   destID,
		Price:           req.Price,
		Currency:        req.Currency,
		DurationNights:  req.DurationNights,
		IncludesFlight:  req.IncludesFlight,
		IncludesHotel:   req.IncludesHotel,
		IncludesMeal:    req.IncludesMeal,
		ImageURL:        req.ImageURL,
		IsActive:        req.IsActive,
	}
	if err := h.voucherSvc.CreateVoucher(c.Request.Context(), v); err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, model.Response{Success: true, Data: v})
}

func (h *VoucherHandler) update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: "invalid id"})
		return
	}
	var req createVoucherRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: err.Error()})
		return
	}
	v, err := h.voucherSvc.GetVoucher(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, model.Response{Success: false, Error: err.Error()})
		return
	}
	v.Title = req.Title
	v.Description = req.Description
	v.DestinationID, _ = uuid.Parse(req.DestinationID)
	v.Price = req.Price
	v.Currency = req.Currency
	v.DurationNights = req.DurationNights
	v.IncludesFlight = req.IncludesFlight
	v.IncludesHotel = req.IncludesHotel
	v.IncludesMeal = req.IncludesMeal
	v.ImageURL = req.ImageURL
	v.IsActive = req.IsActive
	if err := h.voucherSvc.UpdateVoucher(c.Request.Context(), v); err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: v})
}

func (h *VoucherHandler) delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: "invalid id"})
		return
	}
	if err := h.voucherSvc.DeleteVoucher(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: gin.H{"message": "deleted"}})
}

var _ = strconv.Itoa

