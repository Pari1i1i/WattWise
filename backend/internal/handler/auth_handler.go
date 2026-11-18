package handler

import (
	"net/http"

	"github.com/Pari1i1i/WattWise/backend/internal/domain"
	"github.com/Pari1i1i/WattWise/backend/internal/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AuthHandler struct {
	authService *service.AuthService
	db          *gorm.DB
}

func NewAuthHandler(authService *service.AuthService, db *gorm.DB) *AuthHandler {
	return &AuthHandler{authService: authService, db: db}
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req struct {
		Name     string `json:"name" binding:"required"`
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required,min=6"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Input tidak valid: " + err.Error()})
		return
	}

	res, err := h.authService.Register(req.Name, req.Email, req.Password, c.Request.UserAgent(), c.ClientIP())
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"status": "success", "data": res})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Input tidak valid"})
		return
	}

	res, err := h.authService.Login(req.Email, req.Password, c.Request.UserAgent(), c.ClientIP())
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"status": "error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success", "data": res})
}

func (h *AuthHandler) GoogleLogin(c *gin.Context) {
	var req struct {
		IDToken string `json:"id_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "id_token Google diperlukan"})
		return
	}

	res, err := h.authService.LoginWithGoogle(req.IDToken, c.Request.UserAgent(), c.ClientIP())
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"status": "error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success", "data": res})
}

func (h *AuthHandler) Me(c *gin.Context) {
	userID := c.GetString("user_id")
	var user domain.User
	if err := h.db.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "User tidak ditemukan"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success", "data": user})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	sessionID := c.GetString("session_id")
	_ = h.authService.RevokeSession(sessionID)
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Berhasil logout dan sesi telah dicabut"})
}
