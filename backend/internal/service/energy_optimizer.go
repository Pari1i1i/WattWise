package service

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/Pari1i1i/WattWise/backend/internal/domain"
	"gorm.io/datatypes"
)

type Coord3D struct {
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	Z    float64 `json:"z"`
	RotY float64 `json:"rotY"`
}

type ApplianceOptimizationResult struct {
	ApplianceID         string  `json:"appliance_id"`
	Name                string  `json:"name"`
	Category            string  `json:"category"`
	ActiveWatt          int     `json:"active_watt"`
	BeforePosition      Coord3D `json:"before_position"`
	AfterPosition       Coord3D `json:"after_position"`
	ThermalPenaltyBefore float64 `json:"thermal_penalty_before_pct"`
	ThermalPenaltyAfter  float64 `json:"thermal_penalty_after_pct"`
	MonthlyKwhBefore    float64 `json:"monthly_kwh_before"`
	MonthlyKwhAfter     float64 `json:"monthly_kwh_after"`
	Reasoning           string  `json:"reasoning"`
}

type RoomOptimizationSummary struct {
	RoomID                   string                        `json:"room_id"`
	EfficiencyScoreBefore    int                           `json:"efficiency_score_before"`
	EfficiencyScoreAfter     int                           `json:"efficiency_score_after"`
	MonthlyKwhBefore         float64                       `json:"monthly_kwh_before"`
	MonthlyKwhAfter          float64                       `json:"monthly_kwh_after"`
	EstimatedMonthlySavings  float64                       `json:"estimated_monthly_savings_idr"`
	Co2ReducedKg             float64                       `json:"co2_reduced_kg"`
	AppliancesLayout         []ApplianceOptimizationResult `json:"appliances_layout"`
	RecommendedUpgrades      []EcoUpgradeRecommendation    `json:"recommended_upgrades"`
}

// OptimizeRoomLayout menjalankan perhitungan fisika termal sederhana & optimasi tata letak 3D
func OptimizeRoomLayout(room domain.Room, appliances []domain.RoomAppliance) RoomOptimizationSummary {
	// Tentukan pusat sumber panas jendela berdasarkan orientasi kamar
	windowX, windowZ := room.WidthM/2.0, 0.0 // Default jendela di dinding Z=0
	heatMultiplier := 1.0
	switch room.WindowOrientation {
	case "WEST":
		heatMultiplier = 1.35 // Matahari sore (paling panas)
	case "EAST":
		heatMultiplier = 1.20 // Matahari pagi
	case "NORTH", "SOUTH":
		heatMultiplier = 0.85
	default:
		heatMultiplier = 0.50
	}

	var totalKwhBefore, totalKwhAfter float64
	var layoutResults []ApplianceOptimizationResult
	var upgrades []EcoUpgradeRecommendation

	for i, app := range appliances {
		var pos Coord3D
		_ = json.Unmarshal(app.Position3D, &pos)

		// Hitung jarak Euclidean alat ke sumber panas jendela
		distToWindow := math.Sqrt(math.Pow(pos.X-windowX, 2) + math.Pow(pos.Z-windowZ, 2))

		// 1. Hitung Thermal Penalty & Suhu AC sebelum optimasi
		thermalPenaltyBefore := 0.0
		if (app.Category == "COOLING" || app.Category == "REFRIGERATION") && distToWindow < 1.8 {
			thermalPenaltyBefore = math.Round(((1.8-distToWindow)/1.8)*22.0*heatMultiplier*10) / 10
		}

		acTempPenaltyBefore := 0.0
		if app.Category == "COOLING" && app.AcTempSetting < 24 {
			acTempPenaltyBefore = float64(24-app.AcTempSetting) * 6.0 // +6% beban tiap 1°C di bawah 24°C
		}

		// 2. Cari Koordinat Optimal (Sudut terjauh dari jendela panas & tidak tabrakan)
		optPos := pos
		reasoning := "Posisi sudah cukup efisien dan aman dari paparan radiasi panas langsung."
		thermalPenaltyAfter := thermalPenaltyBefore

		if thermalPenaltyBefore > 3.0 {
			// Pindahkan ke area dinding belakang (Z mendekati LengthM) dengan offset per item
			optPos.X = math.Min(room.WidthM-0.5, 0.5+float64(i)*0.8)
			optPos.Z = math.Max(0.5, room.LengthM-0.4)
			optPos.RotY = 180
			thermalPenaltyAfter = 0.0
			reasoning = fmt.Sprintf(
				"Dipindahkan menjauhi jendela %s (jarak %.1fm -> %.1fm) untuk memangkas beban termal kompresor sebesar %.1f%%.",
				room.WindowOrientation, distToWindow, room.LengthM-0.4, thermalPenaltyBefore,
			)
		} else if app.Category == "COOLING" && app.AcTempSetting < 24 {
			reasoning = fmt.Sprintf(
				"Disarankan menaikkan suhu AC dari %d°C ke 24°C dan mengaktifkan sleep timer untuk memangkas beban kompresor %.0f%%.",
				app.AcTempSetting, acTempPenaltyBefore,
			)
		}

		// 3. Kalkulasi kWh Bulanan (Sebelum vs Sesudah)
		standbyHours := math.Max(0, 24.0-app.DailyUsageHours)
		vampireWhBefore := 0.0
		if app.AlwaysPluggedIn {
			vampireWhBefore = app.StandbyWatt * standbyHours
		}

		activeWhBefore := float64(app.ActiveWatt) * (1.0 + (thermalPenaltyBefore+acTempPenaltyBefore)/100.0) * app.DailyUsageHours
		kwhBefore := ((activeWhBefore + vampireWhBefore) * 30.0) / 1000.0

		// Sesudah optimasi: penalti termal hilang, suhu AC ideal (24C), & vampire power diputus
		activeWhAfter := float64(app.ActiveWatt) * (1.0 + thermalPenaltyAfter/100.0) * app.DailyUsageHours
		kwhAfter := (activeWhAfter * 30.0) / 1000.0

		totalKwhBefore += kwhBefore
		totalKwhAfter += kwhAfter

		layoutResults = append(layoutResults, ApplianceOptimizationResult{
			ApplianceID:          app.ID.String(),
			Name:                 app.CustomName,
			Category:             app.Category,
			ActiveWatt:           app.ActiveWatt,
			BeforePosition:       pos,
			AfterPosition:        optPos,
			ThermalPenaltyBefore: thermalPenaltyBefore,
			ThermalPenaltyAfter:  thermalPenaltyAfter,
			MonthlyKwhBefore:     math.Round(kwhBefore*100) / 100,
			MonthlyKwhAfter:      math.Round(kwhAfter*100) / 100,
			Reasoning:            reasoning,
		})

		// Simpan perubahan koordinat ke struct jika perlu di-save ke DB
		optJSON, _ := json.Marshal(optPos)
		appliances[i].OptimizedPosition3D = datatypes.JSON(optJSON)
		appliances[i].ThermalPenaltyPct = thermalPenaltyBefore
	}

	// 4. Rekomendasi Produk Eco-Upgrade + Link Tokopedia Dinamis
	if room.WindowOrientation == "WEST" || room.WindowOrientation == "EAST" {
		price := 125000.0
		savings := math.Max(25000.0, (totalKwhBefore-totalKwhAfter)*room.TariffPerKwh*0.45)
		upgrades = append(upgrades, EcoUpgradeRecommendation{
			ItemName:            "Tirai Blackout Thermal Penahan UV",
			Reason:              fmt.Sprintf("Menahan panas matahari dari jendela arah %s agar suhu kamar turun 2-3°C.", room.WindowOrientation),
			EstimatedPriceIdr:   price,
			MonthlySavingsIdr:   math.Round(savings),
			PaybackPeriodMonths: math.Round((price/savings)*10) / 10,
			TokopediaSearchURL:  BuildTokopediaURL("Tirai Blackout Thermal Anti UV", price),
		})
	}

	pricePlug := 95000.0
	savingsPlug := math.Max(18000.0, (totalKwhBefore-totalKwhAfter)*room.TariffPerKwh*0.35)
	upgrades = append(upgrades, EcoUpgradeRecommendation{
		ItemName:            "Smart Plug Wi-Fi Energy Monitoring",
		Reason:              "Memutus arus standby (Vampire Power) otomatis saat perangkat tidak dipakai atau jam tidur.",
		EstimatedPriceIdr:   pricePlug,
		MonthlySavingsIdr:   math.Round(savingsPlug),
		PaybackPeriodMonths: math.Round((pricePlug/savingsPlug)*10) / 10,
		TokopediaSearchURL:  BuildTokopediaURL("Smart Plug Wifi Energy Monitoring", pricePlug),
	})

	kwhSaved := math.Max(0, totalKwhBefore-totalKwhAfter)
	savingsIdr := math.Round(kwhSaved * room.TariffPerKwh)
	co2Saved := math.Round(kwhSaved*0.85*100) / 100 // Faktor emisi grid Jawa-Madura-Bali ~0.85 kg CO2/kWh

	scoreBefore := 85 - int(math.Min(45, (kwhSaved/math.Max(1, totalKwhBefore))*100))
	scoreAfter := int(math.Min(98, float64(scoreBefore)+24))

	return RoomOptimizationSummary{
		RoomID:                  room.ID.String(),
		EfficiencyScoreBefore:   scoreBefore,
		EfficiencyScoreAfter:    scoreAfter,
		MonthlyKwhBefore:        math.Round(totalKwhBefore*100) / 100,
		MonthlyKwhAfter:         math.Round(totalKwhAfter*100) / 100,
		EstimatedMonthlySavings: savingsIdr,
		Co2ReducedKg:            co2Saved,
		AppliancesLayout:        layoutResults,
		RecommendedUpgrades:     upgrades,
	}
}
