package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go-travel/internal/model"
	"go-travel/internal/service"
)

type ReviewHandler struct {
	reviewSvc *service.ReviewService
}

func NewReviewHandler(reviewSvc *service.ReviewService) *ReviewHandler {
	return &ReviewHandler{reviewSvc: reviewSvc}
}

func (h *ReviewHandler) RegisterRoutes(r *gin.RouterGroup) {
	reviews := r.Group("/reviews")
	{
		reviews.POST("", h.add)
		reviews.GET("/tour/:tour_id", h.list)
	}
}

type addReviewRequest struct {
	TourID  string `json:"tour_id" validate:"required"`
	Rating  int    `json:"rating" validate:"min=1,max=5"`
	Comment string `json:"comment"`
}

func (h *ReviewHandler) add(c *gin.Context) {
	userIDRaw, _ := c.Get("user_id")
	userIDStr, _ := userIDRaw.(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: "invalid user_id"})
		return
	}
	var req addReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: err.Error()})
		return
	}
	tourID, err := uuid.Parse(req.TourID)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: "invalid tour_id"})
		return
	}
	rv := &model.Review{
		TourID:  tourID,
		UserID:  userID,
		Rating:  req.Rating,
		Comment: req.Comment,
	}
	if err := h.reviewSvc.AddReview(c.Request.Context(), rv); err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, model.Response{Success: true, Data: rv})
}

func (h *ReviewHandler) list(c *gin.Context) {
	tourID, err := uuid.Parse(c.Param("tour_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: "invalid tour_id"})
		return
	}
	reviews, err := h.reviewSvc.ListReviews(c.Request.Context(), tourID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: reviews})
}
