package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type User struct {
	ID           uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	Name         string    `gorm:"size:100;not null" json:"name"`
	Email        string    `gorm:"size:150;uniqueIndex;not null" json:"email"`
	PasswordHash *string   `json:"-"` // Boleh NULL kalau login via Google
	GoogleID     *string   `gorm:"size:100;uniqueIndex" json:"google_id,omitempty"`
	AvatarURL    string    `gorm:"type:text" json:"avatar_url,omitempty"`
	AuthProvider string    `gorm:"size:20;default:'LOCAL'" json:"auth_provider"` // 'LOCAL' atau 'GOOGLE'
	EcoPoints    int       `gorm:"default:0" json:"eco_points"`
	CreatedAt    time.Time `json:"created_at"`
}

type UserSession struct {
	ID           uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	UserID       uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`
	RefreshToken string    `gorm:"uniqueIndex;not null" json:"-"`
	UserAgent    string    `json:"user_agent"`
	ClientIP     string    `gorm:"size:45" json:"client_ip"`
	ExpiresAt    time.Time `gorm:"not null" json:"expires_at"`
	IsRevoked    bool      `gorm:"default:false" json:"is_revoked"`
	CreatedAt    time.Time `json:"created_at"`
}

type HouseGroup struct {
	ID           uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	Name         string    `gorm:"size:100;not null" json:"name"`
	InviteCode   string    `gorm:"size:12;uniqueIndex;not null" json:"invite_code"`
	TariffPerKwh float64   `gorm:"type:numeric(10,2);default:1444.70" json:"tariff_per_kwh"`
	VaCategory   string    `gorm:"size:20;default:'1300VA'" json:"va_category"`
	CreatedBy    uuid.UUID `gorm:"type:uuid" json:"created_by"`
	CreatedAt    time.Time `json:"created_at"`
}

type Room struct {
	ID                 uuid.UUID       `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	UserID             uuid.UUID       `gorm:"type:uuid;not null;index" json:"user_id"`
	HouseGroupID       *uuid.UUID      `gorm:"type:uuid;index" json:"house_group_id,omitempty"`
	Name               string          `gorm:"size:100;not null" json:"name"`
	LengthM            float64         `gorm:"type:numeric(4,2);not null" json:"length_m"`
	WidthM             float64         `gorm:"type:numeric(4,2);not null" json:"width_m"`
	HeightM            float64         `gorm:"type:numeric(4,2);default:3.00" json:"height_m"`
	WindowOrientation  string          `gorm:"size:10;default:'WEST'" json:"window_orientation"`
	TariffPerKwh       float64         `gorm:"type:numeric(10,2);default:1444.70" json:"tariff_per_kwh"`
	StructuralFeatures datatypes.JSON  `gorm:"type:jsonb;default:'{}'" json:"structural_features"`
	EfficiencyScore    int             `gorm:"default:0" json:"efficiency_score"`
	Appliances         []RoomAppliance `gorm:"foreignKey:RoomID" json:"appliances,omitempty"`
	CreatedAt          time.Time       `json:"created_at"`
}

type RoomAppliance struct {
	ID                  uuid.UUID      `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	RoomID              uuid.UUID      `gorm:"type:uuid;not null;index" json:"room_id"`
	CustomName          string         `gorm:"size:120;not null" json:"custom_name"`
	Category            string         `gorm:"size:50;default:'OTHER'" json:"category"`
	ActiveWatt          int            `gorm:"not null" json:"active_watt"`
	StandbyWatt         float64        `gorm:"type:numeric(5,2);default:1.0" json:"standby_watt"`
	DailyUsageHours     float64        `gorm:"type:numeric(4,2);not null" json:"daily_usage_hours"`
	AlwaysPluggedIn     bool           `gorm:"default:true" json:"always_plugged_in"`
	AcTempSetting       int            `gorm:"default:24" json:"ac_temp_setting"`
	DimensionsCm        datatypes.JSON `gorm:"type:jsonb;default:'{\"w\":50,\"d\":50,\"h\":50}'" json:"dimensions_cm"`
	Position3D          datatypes.JSON `gorm:"type:jsonb;not null" json:"position_3d"`
	OptimizedPosition3D datatypes.JSON `gorm:"type:jsonb" json:"optimized_position_3d,omitempty"`
	ThermalPenaltyPct   float64        `gorm:"type:numeric(5,2);default:0" json:"thermal_penalty_pct"`
}

type HouseBill struct {
	ID           uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	HouseGroupID uuid.UUID `gorm:"type:uuid;not null;index" json:"house_group_id"`
	BillingMonth string    `gorm:"size:7;not null" json:"billing_month"` // Format: "YYYY-MM"
	TotalBillIdr float64   `gorm:"type:numeric(12,2);not null" json:"total_bill_idr"`
	CreatedAt    time.Time `json:"created_at"`
}
