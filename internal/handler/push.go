package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go-travel/internal/model"
	"go-travel/internal/service"
)

type PushHandler struct {
	pushSvc *service.PushService
}

func NewPushHandler(pushSvc *service.PushService) *PushHandler {
	return &PushHandler{pushSvc: pushSvc}
}

func (h *PushHandler) RegisterRoutes(r *gin.RouterGroup) {
	devices := r.Group("/devices")
	{
		devices.POST("/register", h.registerDevice)
		devices.GET("", h.listDevices)
		devices.DELETE("/:token", h.unregisterDevice)
	}
	admin := r.Group("/admin")
	{
		admin.POST("/push", h.sendPush)
		admin.GET("/push", h.pushHistory)
		admin.GET("/push/provider", h.pushProvider)
	}
}

type registerDeviceRequest struct {
	Platform   string `json:"platform" binding:"required,oneof=ios android web"`
	Token      string `json:"token" binding:"required"`
	AppVersion string `json:"app_version"`
}

func (h *PushHandler) registerDevice(c *gin.Context) {
	userIDStr := userIDFromContext(c)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: "invalid user_id"})
		return
	}
	var req registerDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: err.Error()})
		return
	}
	d := &model.DeviceToken{
		ID:         uuid.New(),
		UserID:     userID,
		Platform:   req.Platform,
		Token:      req.Token,
		AppVersion: req.AppVersion,
		IsActive:   true,
	}
	if err := h.pushSvc.RegisterDevice(c.Request.Context(), d); err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, model.Response{Success: true, Data: d})
}

func (h *PushHandler) listDevices(c *gin.Context) {
	userIDStr := userIDFromContext(c)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: "invalid user_id"})
		return
	}
	items, err := h.pushSvc.Devices(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: items})
}

func (h *PushHandler) unregisterDevice(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: "token required"})
		return
	}
	if err := h.pushSvc.UnregisterDevice(c.Request.Context(), token); err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: gin.H{"message": "device unregistered"}})
}

type sendPushRequest struct {
	UserID   string            `json:"user_id"`
	Platform string            `json:"platform" binding:"omitempty,oneof=ios android web"`
	Title    string            `json:"title" binding:"required"`
	Body     string            `json:"body" binding:"required"`
	Data     map[string]string `json:"data"`
}

func (h *PushHandler) sendPush(c *gin.Context) {
	var req sendPushRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: err.Error()})
		return
	}
	var userID *uuid.UUID
	if req.UserID != "" {
		id, err := uuid.Parse(req.UserID)
		if err != nil {
			c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: "invalid user_id"})
			return
		}
		userID = &id
	}
	n, err := h.pushSvc.Send(c.Request.Context(), userID, req.Platform, req.Title, req.Body, req.Data)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: n})
}

func (h *PushHandler) pushHistory(c *gin.Context) {
	items, err := h.pushSvc.History(c.Request.Context(), 100)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: items})
}

func (h *PushHandler) pushProvider(c *gin.Context) {
	c.JSON(http.StatusOK, model.Response{Success: true, Data: gin.H{"provider": h.pushSvc.Provider()}})
}
