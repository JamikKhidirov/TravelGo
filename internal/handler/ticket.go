package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go-travel/internal/model"
	"go-travel/internal/service"
)

type TicketHandler struct {
	ticketSvc  *service.TicketService
	travelSvc  *service.TravelService
	voucherSvc *service.VoucherService
}

func NewTicketHandler(ticketSvc *service.TicketService, travelSvc *service.TravelService, voucherSvc *service.VoucherService) *TicketHandler {
	return &TicketHandler{ticketSvc: ticketSvc, travelSvc: travelSvc, voucherSvc: voucherSvc}
}

func (h *TicketHandler) RegisterRoutes(r *gin.RouterGroup) {
	tickets := r.Group("/tickets")
	{
		tickets.GET("", h.list)
		tickets.GET("/search", h.search)
		tickets.GET("/:id", h.get)
	}
	r.GET("/cities/:city/packages", h.cityPackages)
}

func (h *TicketHandler) list(c *gin.Context) {
	var p model.Pagination
	_ = c.ShouldBindQuery(&p)
	var from *time.Time
	if v := c.Query("from"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: "from must be RFC3339"})
			return
		}
		from = &t
	}
	items, total, err := h.ticketSvc.List(c.Request.Context(), c.Query("type"), c.Query("origin"), c.Query("destination"), from, &p)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: items, Meta: model.PaginationResponse{Page: p.Page, PageSize: p.PageSize, Total: total, TotalPages: pageCount(total, p.PageSize)}})
}

func (h *TicketHandler) search(c *gin.Context) {
	var p model.Pagination
	_ = c.ShouldBindQuery(&p)
	items, total, err := h.ticketSvc.Search(c.Request.Context(), c.Query("q"), c.Query("type"), &p)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: items, Meta: model.PaginationResponse{Page: p.Page, PageSize: p.PageSize, Total: total, TotalPages: pageCount(total, p.PageSize)}})
}

func (h *TicketHandler) get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: "invalid id"})
		return
	}
	t, err := h.ticketSvc.Get(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: t})
}

func (h *TicketHandler) cityPackages(c *gin.Context) {
	city := c.Param("city")
	p := &model.Pagination{Page: 1, PageSize: 20}
	dests, _, _ := h.travelSvc.SearchDestinations(c.Request.Context(), city, "", "", 0, 0, p)
	tours, _, _ := h.travelSvc.SearchTours(c.Request.Context(), city, nil, nil, 0, 0, 0, 0, p)
	vouchers, _ := h.voucherSvc.ByCity(c.Request.Context(), city, 20)
	tickets, _ := h.ticketSvc.ByCity(c.Request.Context(), city, 20)
	c.JSON(http.StatusOK, model.Response{Success: true, Data: gin.H{
		"city":         city,
		"destinations": dests,
		"tours":        tours,
		"vouchers":     vouchers,
		"tickets":      tickets,
	}})
}

func pageCount(total, size int) int {
	if size <= 0 {
		size = 20
	}
	if total == 0 {
		return 0
	}
	return (total + size - 1) / size
}
