package handler

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/Pari1i1i/WattWise/backend/internal/domain"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type HouseHandler struct {
	db *gorm.DB
}

func NewHouseHandler(db *gorm.DB) *HouseHandler {
	return &HouseHandler{db: db}
}

func (h *HouseHandler) CreateHouseGroup(c *gin.Context) {
	var req struct {
		Name         string  `json:"name" binding:"required"`
		VaCategory   string  `json:"va_category"`
		TariffPerKwh float64 `json:"tariff_per_kwh"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	userID, _ := uuid.Parse(c.GetString("user_id"))
	if req.TariffPerKwh <= 0 {
		req.TariffPerKwh = 1444.70
	}
	if req.VaCategory == "" {
		req.VaCategory = "1300VA"
	}

	inviteCode := "KOS-" + strings.ToUpper(uuid.New().String()[:6])
	house := domain.HouseGroup{
		Name:         req.Name,
		InviteCode:   inviteCode,
		TariffPerKwh: req.TariffPerKwh,
		VaCategory:   req.VaCategory,
		CreatedBy:    userID,
	}

	if err := h.db.Create(&house).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal membuat grup kontrakan/kos"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"status": "success", "data": house})
}

func (h *HouseHandler) JoinHouseGroup(c *gin.Context) {
	var req struct {
		InviteCode string `json:"invite_code" binding:"required"`
		RoomID     string `json:"room_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	var house domain.HouseGroup
	if err := h.db.Where("invite_code = ?", strings.ToUpper(req.InviteCode)).First(&house).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Kode undangan kos tidak ditemukan"})
		return
	}

	userID := c.GetString("user_id")
	res := h.db.Model(&domain.Room{}).
		Where("id = ? AND user_id = ?", req.RoomID, userID).
		Updates(map[string]interface{}{
			"house_group_id": house.ID,
			"tariff_per_kwh": house.TariffPerKwh,
		})

	if res.Error != nil || res.RowsAffected == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Kamar tidak valid atau bukan milik Anda"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("Kamar berhasil tergabung ke %s", house.Name),
		"data":    house,
	})
}

func (h *HouseHandler) CreateMonthlyBill(c *gin.Context) {
	houseID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "ID grup tidak valid"})
		return
	}

	var req struct {
		BillingMonth string  `json:"billing_month" binding:"required"` // Contoh: "2026-11"
		TotalBillIdr float64 `json:"total_bill_idr" binding:"required,gt=0"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	bill := domain.HouseBill{
		HouseGroupID: houseID,
		BillingMonth: req.BillingMonth,
		TotalBillIdr: req.TotalBillIdr,
	}
	if err := h.db.Create(&bill).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan tagihan"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"status": "success", "data": bill})
}

type SplitBillRow struct {
	RoomID          string  `json:"room_id"`
	RoomName        string  `json:"room_name"`
	OwnerName       string  `json:"owner_name"`
	ApplianceCount  int     `json:"appliance_count"`
	EstimatedKwh    float64 `json:"estimated_kwh"`
	UsagePercentage float64 `json:"usage_percentage"`
	FairBillShare   float64 `json:"fair_bill_share_idr"`
}

// GetFairSplitBill mengeksekusi query SQL CTE & Window Function untuk membagi tagihan proporsional per kamar
func (h *HouseHandler) GetFairSplitBill(c *gin.Context) {
	houseID := c.Param("id")
	billingMonth := c.DefaultQuery("month", "2026-11")

	var house domain.HouseGroup
	if err := h.db.Where("id = ?", houseID).First(&house).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Grup kos/kontrakan tidak ditemukan"})
		return
	}

	var bill domain.HouseBill
	totalBill := 0.0
	if err := h.db.Where("house_group_id = ? AND billing_month = ?", houseID, billingMonth).Order("created_at desc").First(&bill).Error; err == nil {
		totalBill = bill.TotalBillIdr
	}

	query := `
	WITH room_consumption AS (
		SELECT 
			r.id AS room_id,
			r.name AS room_name,
			u.name AS owner_name,
			COUNT(ra.id) AS appliance_count,
			COALESCE(SUM(
				(
					(ra.active_watt * (1.0 + (ra.thermal_penalty_pct / 100.0)) * ra.daily_usage_hours) +
					(CASE WHEN ra.always_plugged_in THEN ra.standby_watt * GREATEST(0, 24.0 - ra.daily_usage_hours) ELSE 0 END)
				) * 30.0 / 1000.0
			), 0) AS monthly_kwh
		FROM rooms r
		JOIN users u ON u.id = r.user_id
		LEFT JOIN room_appliances ra ON ra.room_id = r.id
		WHERE r.house_group_id = ?
		GROUP BY r.id, r.name, u.name
	)
	SELECT 
		rc.room_id,
		rc.room_name,
		rc.owner_name,
		rc.appliance_count,
		ROUND(rc.monthly_kwh::numeric, 2) AS estimated_kwh,
		ROUND((rc.monthly_kwh / NULLIF(SUM(rc.monthly_kwh) OVER (), 0) * 100.0)::numeric, 2) AS usage_percentage,
		ROUND((
			CASE 
				WHEN ? > 0 THEN (rc.monthly_kwh / NULLIF(SUM(rc.monthly_kwh) OVER (), 0)) * ?
				ELSE rc.monthly_kwh * ?
			END
		)::numeric, 0) AS fair_bill_share
	FROM room_consumption rc
	ORDER BY estimated_kwh DESC;
	`

	var rows []SplitBillRow
	if err := h.db.Raw(query, houseID, totalBill, totalBill, house.TariffPerKwh).Scan(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menghitung split bill: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"house_group":     house,
			"billing_month":   billingMonth,
			"actual_bill_idr": totalBill,
			"split_breakdown": rows,
		},
	})
}
