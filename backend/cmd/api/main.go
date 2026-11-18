package main

import (
	"log"
	"net/http"
	"os"

	"github.com/Pari1i1i/WattWise/backend/internal/config"
	"github.com/Pari1i1i/WattWise/backend/internal/domain"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load("../.env"); err != nil {
		_ = godotenv.Load()
	}

	db := config.ConnectDB()
	sqlDB, _ := db.DB()

	// AutoMigrate semua tabel ke PostgreSQL
	if err := db.AutoMigrate(
		&domain.User{},
		&domain.UserSession{},
		&domain.HouseGroup{},
		&domain.Room{},
		&domain.RoomAppliance{},
	); err != nil {
		log.Fatalf("❌ Gagal migrasi database: %v", err)
	}
	log.Println("✅ AutoMigrate tabel berhasil!")

	r := gin.Default()

	v1 := r.Group("/api/v1")
	{
		v1.GET("/health", func(c *gin.Context) {
			if err := sqlDB.Ping(); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "db": "disconnected"})
				return
			}
			c.JSON(http.StatusOK, gin.H{
				"status":  "success",
				"service": "WattWise Backend API",
				"db":      "connected",
			})
		})
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	r.Run(":" + port)
}
