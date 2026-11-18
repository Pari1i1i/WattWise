package handler

import (
	"encoding/json"
	"net/http"

	"github.com/Pari1i1i/WattWise/backend/internal/domain"
	"github.com/Pari1i1i/WattWise/backend/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type RoomHandler struct {
	db *gorm.DB
}

func NewRoomHandler(db *gorm.DB) *RoomHandler {
	return &RoomHandler{db: db}
}

type CreateRoomRequest struct {
	Name              string  `json:"name" binding:"required"`
	LengthM           float64 `json:"length_m" binding:"required,gt=0"`
	WidthM            float64 `json:"width_m" binding:"required,gt=0"`
	HeightM           float64 `json:"height_m"`
	WindowOrientation string  `json:"window_orientation"`
	TariffPerKwh      float64 `json:"tariff_per_kwh"`
	Appliances        []struct {
		CustomName      string          `json:"custom_name" binding:"required"`
		Category        string          `json:"category"`
		ActiveWatt      int             `json:"active_watt" binding:"required,gt=0"`
		StandbyWatt     float64         `json:"standby_watt"`
		DailyUsageHours float64         `json:"daily_usage_hours" binding:"required,gt=0"`
		AlwaysPluggedIn bool            `json:"always_plugged_in"`
		AcTempSetting   int             `json:"ac_temp_setting"`
		Position3D      service.Coord3D `json:"position_3d"`
	} `json:"appliances"`
}

func (h *RoomHandler) CreateRoom(c *gin.Context) {
	var req CreateRoomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	userID, _ := uuid.Parse(c.GetString("user_id"))
	if req.TariffPerKwh <= 0 {
		req.TariffPerKwh = 1444.70
	}
	if req.WindowOrientation == "" {
		req.WindowOrientation = "WEST"
	}

	room := domain.Room{
		UserID:            userID,
		Name:              req.Name,
		LengthM:           req.LengthM,
		WidthM:            req.WidthM,
		HeightM:           req.HeightM,
		WindowOrientation: req.WindowOrientation,
		TariffPerKwh:      req.TariffPerKwh,
	}

	if err := h.db.Create(&room).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan kamar"})
		return
	}

	for _, app := range req.Appliances {
		posBytes, _ := json.Marshal(app.Position3D)
		acTemp := app.AcTempSetting
		if acTemp == 0 {
			acTemp = 24
		}
		roomApp := domain.RoomAppliance{
			RoomID:          room.ID,
			CustomName:      app.CustomName,
			Category:        app.Category,
			ActiveWatt:      app.ActiveWatt,
			StandbyWatt:     app.StandbyWatt,
			DailyUsageHours: app.DailyUsageHours,
			AlwaysPluggedIn: app.AlwaysPluggedIn,
			AcTempSetting:   acTemp,
			Position3D:      datatypes.JSON(posBytes),
		}
		h.db.Create(&roomApp)
	}

	h.db.Preload("Appliances").First(&room, "id = ?", room.ID)
	c.JSON(http.StatusCreated, gin.H{"status": "success", "data": room})
}

func (h *RoomHandler) GetMyRooms(c *gin.Context) {
	userID := c.GetString("user_id")
	var rooms []domain.Room
	h.db.Preload("Appliances").Where("user_id = ?", userID).Order("created_at desc").Find(&rooms)
	c.JSON(http.StatusOK, gin.H{"status": "success", "data": rooms})
}

func (h *RoomHandler) OptimizeRoom(c *gin.Context) {
	roomID := c.Param("id")
	userID := c.GetString("user_id")

	var room domain.Room
	if err := h.db.Preload("Appliances").Where("id = ? AND user_id = ?", roomID, userID).First(&room).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Kamar tidak ditemukan"})
		return
	}

	summary := service.OptimizeRoomLayout(room, room.Appliances)

	// Update skor efisiensi & koordinat optimal ke DB
	h.db.Model(&room).Update("efficiency_score", summary.EfficiencyScoreAfter)
	for _, app := range room.Appliances {
		h.db.Model(&domain.RoomAppliance{}).Where("id = ?", app.ID).Updates(map[string]interface{}{
			"optimized_position_3d": app.OptimizedPosition3D,
			"thermal_penalty_pct":   app.ThermalPenaltyPct,
		})
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": summary})
}

// SandboxOptimize bisa diakses tanpa login untuk Demo Interaktif di Landing Page
func (h *RoomHandler) SandboxOptimize(c *gin.Context) {
	var req CreateRoomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	if req.TariffPerKwh <= 0 {
		req.TariffPerKwh = 1444.70
	}
	room := domain.Room{
		ID:                uuid.New(),
		Name:              req.Name,
		LengthM:           req.LengthM,
		WidthM:            req.WidthM,
		WindowOrientation: req.WindowOrientation,
		TariffPerKwh:      req.TariffPerKwh,
	}

	var apps []domain.RoomAppliance
	for _, a := range req.Appliances {
		posBytes, _ := json.Marshal(a.Position3D)
		apps = append(apps, domain.RoomAppliance{
			ID:              uuid.New(),
			CustomName:      a.CustomName,
			Category:        a.Category,
			ActiveWatt:      a.ActiveWatt,
			StandbyWatt:     a.StandbyWatt,
			DailyUsageHours: a.DailyUsageHours,
			AlwaysPluggedIn: a.AlwaysPluggedIn,
			AcTempSetting:   a.AcTempSetting,
			Position3D:      datatypes.JSON(posBytes),
		})
	}

	summary := service.OptimizeRoomLayout(room, apps)
	c.JSON(http.StatusOK, gin.H{"status": "success", "data": summary})
}
