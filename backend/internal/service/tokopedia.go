package service

import (
	"fmt"
	"net/url"
)

type EcoUpgradeRecommendation struct {
	ItemName            string  `json:"item_name"`
	Reason              string  `json:"reason"`
	EstimatedPriceIdr   float64 `json:"estimated_price_idr"`
	MonthlySavingsIdr   float64 `json:"monthly_savings_idr"`
	PaybackPeriodMonths float64 `json:"payback_period_months"`
	TokopediaSearchURL  string  `json:"tokopedia_search_url"`
}

// BuildTokopediaURL merakit link pencarian Tokopedia otomatis dengan filter harga & ulasan terbaik (ob=5)
func BuildTokopediaURL(keyword string, estimatedPrice float64) string {
	minPrice := int(estimatedPrice * 0.7) // Batas bawah 70% dari estimasi
	maxPrice := int(estimatedPrice * 1.3) // Batas atas 130% dari estimasi

	return fmt.Sprintf(
		"https://www.tokopedia.com/search?st=product&q=%s&pmin=%d&pmax=%d&ob=5",
		url.QueryEscape(keyword),
		minPrice,
		maxPrice,
	)
}
