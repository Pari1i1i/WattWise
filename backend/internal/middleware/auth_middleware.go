package middleware

import (
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Pari1i1i/WattWise/backend/internal/domain"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

func AuthMiddleware(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.JSON(http.StatusUnauthorized, gin.H{"status": "error", "message": "Token autentikasi diperlukan"})
			c.Abort()
			return
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		secret := os.Getenv("JWT_SECRET")
		if secret == "" {
			secret = "wattwise-default-secret-key"
		}

		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			return []byte(secret), nil
		})

		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"status": "error", "message": "Token tidak valid atau kedaluwarsa"})
			c.Abort()
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"status": "error", "message": "Klaim token tidak valid"})
			c.Abort()
			return
		}

		sessionID, _ := claims["session_id"].(string)
		userID, _ := claims["sub"].(string)

		// Verifikasi sesi aktif di database (Poin Keamanan Session)
		var session domain.UserSession
		if err := db.Where("id = ? AND is_revoked = false AND expires_at > ?", sessionID, time.Now()).First(&session).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"status": "error", "message": "Sesi telah berakhir atau telah logout"})
			c.Abort()
			return
		}

		c.Set("user_id", userID)
		c.Set("session_id", sessionID)
		c.Next()
	}
}
