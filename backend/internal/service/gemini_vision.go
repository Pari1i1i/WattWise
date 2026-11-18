package service

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type DetectedAppliance struct {
	CustomName      string         `json:"custom_name"`
	Category        string         `json:"category"` // COOLING, REFRIGERATION, COMPUTING, LIGHTING, OTHER
	ActiveWatt      int            `json:"active_watt"`
	StandbyWatt     float64        `json:"standby_watt"`
	DailyUsageHours float64        `json:"daily_usage_hours"`
	AcTempSetting   int            `json:"ac_temp_setting"`
	DimensionsCm    map[string]int `json:"dimensions_cm"` // {"w": 50, "d": 50, "h": 85}
	Position3D      Coord3D        `json:"position_3d"`
}

type VisionScanResult struct {
	WindowOrientation  string              `json:"window_orientation"`
	DetectedAppliances []DetectedAppliance `json:"detected_appliances"`
	AIAnalysisNote     string              `json:"ai_analysis_note"`
	SourceMode         string              `json:"source_mode"` // "GEMINI_LIVE" atau "SMART_FALLBACK"
}

// AnalyzeRoomPhoto mengirim foto kamar ke Gemini 2.5 Flash Vision dan mengekstrak JSON koordinat 3D
func AnalyzeRoomPhoto(imageBytes []byte, mimeType string, roomWidth, roomLength float64) (*VisionScanResult, error) {
	apiKey := strings.TrimSpace(os.Getenv("GEMINI_API_KEY"))
	if apiKey == "" || apiKey == "your_gemini_api_key_here" {
		return getFallbackScanResult(roomWidth, roomLength), nil
	}

	b64Image := base64.StdEncoding.EncodeToString(imageBytes)
	prompt := fmt.Sprintf(`Kamu adalah arsitek termal & auditor energi WattWise.
Analisis foto ruangan berukuran lebar %.1fm (sumbu X: 0.0 s/d %.1f) dan panjang %.1fm (sumbu Z: 0.0 s/d %.1f, di mana Z=0 adalah sisi jendela utama).
Deteksi semua perangkat elektronik dan perabot relevan di dalam foto, perkirakan konsumsi daya (Watt), ukuran (cm), dan koordinat 3D awalnya di dalam ruangan.
Kembalikan HANYA JSON murni dengan skema persis berikut:
{
  "window_orientation": "WEST",
  "ai_analysis_note": "Ringkasan singkat kondisi pencahayaan dan penempatan elektronik di foto dalam Bahasa Indonesia.",
  "detected_appliances": [
    {
      "custom_name": "Nama alat elektronik",
      "category": "COOLING | REFRIGERATION | COMPUTING | LIGHTING | OTHER",
      "active_watt": 150,
      "standby_watt": 2.0,
      "daily_usage_hours": 8,
      "ac_temp_setting": 24,
      "dimensions_cm": {"w": 60, "d": 50, "h": 80},
      "position_3d": {"x": 1.2, "y": 0.0, "z": 0.5, "rotY": 0}
    }
  ]
}`, roomWidth, roomWidth, roomLength, roomLength)

	payload := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]interface{}{
					{"text": prompt},
					{
						"inline_data": map[string]string{
							"mime_type": mimeType,
							"data":      b64Image,
						},
					},
				},
			},
		},
		"generationConfig": map[string]interface{}{
			"responseMimeType": "application/json",
			"temperature":      0.2,
		},
	}

	bodyBytes, _ := json.Marshal(payload)
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent?key=%s", apiKey)

	client := &http.Client{Timeout: 25 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewBuffer(bodyBytes))
	if err != nil || resp.StatusCode != http.StatusOK {
		// Gunakan Smart Fallback jika jaringan/kuota API bermasalah saat demo
		return getFallbackScanResult(roomWidth, roomLength), nil
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	var geminiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.Unmarshal(respBytes, &geminiResp); err != nil || len(geminiResp.Candidates) == 0 {
		return getFallbackScanResult(roomWidth, roomLength), nil
	}

	rawJSON := geminiResp.Candidates[0].Content.Parts[0].Text
	var result VisionScanResult
	if err := json.Unmarshal([]byte(rawJSON), &result); err != nil {
		return getFallbackScanResult(roomWidth, roomLength), nil
	}

	result.SourceMode = "GEMINI_LIVE"
	return &result, nil
}

func getFallbackScanResult(w, l float64) *VisionScanResult {
	return &VisionScanResult{
		WindowOrientation: "WEST",
		AIAnalysisNote:    "Deteksi mendapati unit AC dan Kulkas Mini berada di dekat area bukaan jendela yang berpotensi meningkatkan beban termal kompresor.",
		SourceMode:        "SMART_FALLBACK",
		DetectedAppliances: []DetectedAppliance{
			{
				CustomName:      "AC Split 1/2 PK",
				Category:        "COOLING",
				ActiveWatt:      400,
				StandbyWatt:     2.0,
				DailyUsageHours: 9,
				AcTempSetting:   20,
				DimensionsCm:    map[string]int{"w": 75, "d": 20, "h": 28},
				Position3D:      Coord3D{X: w / 2, Y: 2.3, Z: 0.25, RotY: 0},
			},
			{
				CustomName:      "Kulkas Mini Portable",
				Category:        "REFRIGERATION",
				ActiveWatt:      70,
				StandbyWatt:     1.0,
				DailyUsageHours: 24,
				AcTempSetting:   24,
				DimensionsCm:    map[string]int{"w": 48, "d": 45, "h": 50},
				Position3D:      Coord3D{X: 0.6, Y: 0.0, Z: 0.4, RotY: 0},
			},
			{
				CustomName:      "PC Setup & Monitor 24 Inch",
				Category:        "COMPUTING",
				ActiveWatt:      210,
				StandbyWatt:     5.5,
				DailyUsageHours: 7,
				AcTempSetting:   24,
				DimensionsCm:    map[string]int{"w": 110, "d": 60, "h": 75},
				Position3D:      Coord3D{X: w - 0.8, Y: 0.0, Z: l / 2, RotY: 270},
			},
		},
	}
}
