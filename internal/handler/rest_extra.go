package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go-travel/internal/model"
	"go-travel/internal/service"
)

func (h *TravelHandler) searchDestinations(c *gin.Context) {
	var p model.Pagination
	if err := c.ShouldBindQuery(&p); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: err.Error()})
		return
	}
	q := c.Query("q")
	country := c.Query("country")
	city := c.Query("city")
	minRating := 0.0
	maxPrice := 0.0
	if v := c.Query("min_rating"); v != "" {
		fmt.Sscan(v, &minRating)
	}
	if v := c.Query("max_price"); v != "" {
		fmt.Sscan(v, &maxPrice)
	}
	dests, total, err := h.svc.SearchDestinations(c.Request.Context(), q, country, city, minRating, maxPrice, &p)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: dests, Meta: paginationResponse{Page: p.Page, PageSize: p.PageSize, Total: total, TotalPages: (total + p.PageSize - 1) / p.PageSize}})
}

func (h *TravelHandler) topDestinations(c *gin.Context) {
	limit := 10
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	dests, err := h.svc.TopDestinations(c.Request.Context(), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: dests})
}

func (h *TravelHandler) listCountries(c *gin.Context) {
	countries, err := h.svc.ListCountries(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: countries})
}

func (h *TravelHandler) updateDestination(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: "invalid id"})
		return
	}
	var d model.Destination
	if err := c.ShouldBindJSON(&d); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: err.Error()})
		return
	}
	d.ID = id
	if err := h.svc.UpdateDestination(c.Request.Context(), &d); err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: d})
}

func (h *TravelHandler) deleteDestination(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: "invalid id"})
		return
	}
	if err := h.svc.DeleteDestination(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: gin.H{"message": "deleted"}})
}

func (h *TravelHandler) searchTours(c *gin.Context) {
	var p model.Pagination
	if err := c.ShouldBindQuery(&p); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: err.Error()})
		return
	}
	q := c.Query("q")
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
	minPrice := 0.0
	maxPrice := 0.0
	minDuration := 0
	maxDuration := 0
	if v := c.Query("min_price"); v != "" {
		fmt.Sscan(v, &minPrice)
	}
	if v := c.Query("max_price"); v != "" {
		fmt.Sscan(v, &maxPrice)
	}
	if v := c.Query("min_duration"); v != "" {
		fmt.Sscan(v, &minDuration)
	}
	if v := c.Query("max_duration"); v != "" {
		fmt.Sscan(v, &maxDuration)
	}
	tours, total, err := h.svc.SearchTours(c.Request.Context(), q, destID, available, minPrice, maxPrice, minDuration, maxDuration, &p)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: tours, Meta: paginationResponse{Page: p.Page, PageSize: p.PageSize, Total: total, TotalPages: (total + p.PageSize - 1) / p.PageSize}})
}

func (h *TravelHandler) featuredTours(c *gin.Context) {
	var p model.Pagination
	p.Page = 1
	p.PageSize = 10
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			p.PageSize = n
		}
	}
	available := true
	tours, total, err := h.svc.SearchTours(c.Request.Context(), "", nil, &available, 0, 0, 0, 0, &p)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: tours, Meta: paginationResponse{Page: p.Page, PageSize: p.PageSize, Total: total, TotalPages: (total + p.PageSize - 1) / p.PageSize}})
}

func (h *TravelHandler) updateTour(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: "invalid id"})
		return
	}
	var t model.Tour
	if err := c.ShouldBindJSON(&t); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: err.Error()})
		return
	}
	t.ID = id
	if err := h.svc.UpdateTour(c.Request.Context(), &t); err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: t})
}

func (h *TravelHandler) deleteTour(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: "invalid id"})
		return
	}
	if err := h.svc.DeleteTour(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: gin.H{"message": "deleted"}})
}

func (h *TravelHandler) listSyncJobs(c *gin.Context) {
	if h.syncSvc == nil {
		c.JSON(http.StatusOK, model.Response{Success: true, Data: []interface{}{}})
		return
	}
	jobs, err := h.syncSvc.GetJobs(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: jobs})
}

// sync override in rest.go sync method? keep existing
var _ = json.Marshal
var _ = service.NewTravelService
