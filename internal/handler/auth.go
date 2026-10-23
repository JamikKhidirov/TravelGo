package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go-travel/internal/model"
	"go-travel/internal/service"
)

type AuthHandler struct {
	authSvc *service.AuthService
}

func NewAuthHandler(authSvc *service.AuthService) *AuthHandler {
	return &AuthHandler{authSvc: authSvc}
}

func (h *AuthHandler) RegisterRoutes(r *gin.RouterGroup) {
	auth := r.Group("/auth")
	{
		auth.POST("/register", h.register)
		auth.POST("/login", h.login)
		auth.GET("/me", h.me)
		auth.GET("/profile", h.profile)
		auth.PUT("/profile", h.updateProfile)
		auth.POST("/change-password", h.changePassword)
		auth.POST("/refresh", h.refresh)
	}
}

type registerRequest struct {
	Email     string `json:"email" validate:"required,email"`
	Password  string `json:"password" validate:"required,min=6"`
	FirstName string `json:"first_name" validate:"required"`
	LastName  string `json:"last_name" validate:"required"`
	Phone     string `json:"phone"`
	Role      string `json:"role" validate:"omitempty,oneof=user admin"`
}

type loginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

func (h *AuthHandler) register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: err.Error()})
		return
	}
	role := req.Role
	if role == "" {
		role = "user"
	}
	u, token, err := h.authSvc.Register(c.Request.Context(), &model.LoginRequest{Email: req.Email, Password: req.Password}, req.FirstName, req.LastName, req.Phone, role)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, model.Response{Success: true, Data: gin.H{"user": u, "token": token}})
}

func (h *AuthHandler) login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: err.Error()})
		return
	}
	u, token, err := h.authSvc.Login(c.Request.Context(), &model.LoginRequest{Email: req.Email, Password: req.Password})
	if err != nil {
		c.JSON(http.StatusUnauthorized, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: gin.H{"user": u, "token": token}})
}

func (h *AuthHandler) me(c *gin.Context) {
	userID, _ := c.Get("user_id")
	role, _ := c.Get("user_role")
	c.JSON(http.StatusOK, model.Response{Success: true, Data: gin.H{"user_id": userID, "role": role}})
}

func (h *AuthHandler) profile(c *gin.Context) {
	userIDStr := userIDFromContext(c)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: "invalid user_id"})
		return
	}
	u, err := h.authSvc.GetUser(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: u})
}

func (h *AuthHandler) changePassword(c *gin.Context) {
	userIDStr := userIDFromContext(c)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: "invalid user_id"})
		return
	}
	var req struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required,min=6"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: err.Error()})
		return
	}
	if err := h.authSvc.ChangePassword(c.Request.Context(), userID, req.OldPassword, req.NewPassword); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: gin.H{"message": "password changed"}})
}

func (h *AuthHandler) refresh(c *gin.Context) {
	userIDStr := userIDFromContext(c)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: "invalid user_id"})
		return
	}
	role, _ := c.Get("user_role")
	if role == nil || role == "" {
		role = "user"
	}
	token, err := h.authSvc.RefreshToken(c.Request.Context(), userID, role.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: gin.H{"token": token}})
}

func (h *AuthHandler) updateProfile(c *gin.Context) {
	userIDStr := userIDFromContext(c)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: "invalid user_id"})
		return
	}
	var req struct {
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Phone     string `json:"phone"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: err.Error()})
		return
	}
	u, err := h.authSvc.GetUser(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, model.Response{Success: false, Error: err.Error()})
		return
	}
	if req.FirstName != "" {
		u.FirstName = req.FirstName
	}
	if req.LastName != "" {
		u.LastName = req.LastName
	}
	if req.Phone != "" {
		u.Phone = req.Phone
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: u})
}

type CartHandler struct {
	cartSvc *service.CartService
}

func NewCartHandler(cartSvc *service.CartService) *CartHandler {
	return &CartHandler{cartSvc: cartSvc}
}

func (h *CartHandler) RegisterRoutes(r *gin.RouterGroup) {
	cart := r.Group("/cart")
	{
		cart.GET("", h.list)
		cart.POST("/add", h.add)
		cart.DELETE("/item/:item_id", h.remove)
		cart.POST("/checkout", h.checkout)
		cart.POST("/clear", h.clear)
		cart.PUT("/item/:item_id", h.updateItem)
	}
}

type addToCartRequest struct {
	TourID  string  `json:"tour_id" validate:"required"`
	Quantity int    `json:"quantity" validate:"min=1"`
	Price   float64 `json:"price" validate:"gt=0"`
}

func (h *CartHandler) list(c *gin.Context) {
	userIDStr := userIDFromContext(c)
	if userIDStr == "" {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: "user_id required"})
		return
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: "invalid user_id"})
		return
	}
	cart, items, err := h.cartSvc.ListCart(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: gin.H{"cart": cart, "items": items}})
}

func (h *CartHandler) add(c *gin.Context) {
	userIDStr := userIDFromContext(c)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: "invalid user_id"})
		return
	}
	var req addToCartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: err.Error()})
		return
	}
	tourID, err := uuid.Parse(req.TourID)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: "invalid tour_id"})
		return
	}
	item, err := h.cartSvc.AddToCart(c.Request.Context(), userID, tourID, req.Quantity, req.Price)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: item})
}

func (h *CartHandler) remove(c *gin.Context) {
	userIDStr := userIDFromContext(c)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: "invalid user_id"})
		return
	}
	itemID, err := uuid.Parse(c.Param("item_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: "invalid item_id"})
		return
	}
	if err := h.cartSvc.RemoveFromCart(c.Request.Context(), userID, itemID); err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: gin.H{"message": "removed"}})
}

func (h *CartHandler) clear(c *gin.Context) {
	userIDStr := userIDFromContext(c)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: "invalid user_id"})
		return
	}
	if err := h.cartSvc.ClearCart(c.Request.Context(), userID); err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: gin.H{"message": "cleared"}})
}

func (h *CartHandler) updateItem(c *gin.Context) {
	userIDStr := userIDFromContext(c)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: "invalid user_id"})
		return
	}
	itemID, err := uuid.Parse(c.Param("item_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: "invalid item_id"})
		return
	}
	var req struct {
		Quantity int `json:"quantity" binding:"min=1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: err.Error()})
		return
	}
	if err := h.cartSvc.UpdateCartItem(c.Request.Context(), userID, itemID, req.Quantity); err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: gin.H{"message": "updated"}})
}

type checkoutRequest struct {
	TourID         string    `json:"tour_id" validate:"required"`
	DestinationID  string    `json:"destination_id" validate:"required"`
	PassengerName  string    `json:"passenger_name" validate:"required"`
	PassengerEmail string    `json:"passenger_email" validate:"required,email"`
	PassengerPhone string    `json:"passenger_phone"`
	TravelDate     string    `json:"travel_date" validate:"required"`
	Nights         int       `json:"nights" validate:"min=1"`
	Adults         int       `json:"adults" validate:"min=1"`
	Children       int       `json:"children"`
}

func (h *CartHandler) checkout(c *gin.Context) {
	userIDStr := userIDFromContext(c)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: "invalid user_id"})
		return
	}
	var req checkoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: err.Error()})
		return
	}
	tourID, _ := uuid.Parse(req.TourID)
	destID, _ := uuid.Parse(req.DestinationID)
	travelDate, _ := time.Parse(time.RFC3339, req.TravelDate)

	o := &model.Order{
		TourID:         tourID,
		DestinationID:  destID,
		PassengerName:  req.PassengerName,
		PassengerEmail: req.PassengerEmail,
		PassengerPhone: req.PassengerPhone,
		TravelDate:     travelDate,
		Nights:         req.Nights,
		Adults:         req.Adults,
		Children:       req.Children,
	}
	order, err := h.cartSvc.Checkout(c.Request.Context(), userID, o)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: order})
}

var _ = strconv.Itoa
var _ = time.Time{}


func userIDFromContext(c *gin.Context) string {
	if v, ok := c.Get("user_id"); ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	if v := c.GetHeader("X-User-Id"); v != "" {
		return v
	}
	return ""
}



