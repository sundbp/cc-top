package tui

import (
	"strings"
	"testing"

	"github.com/nixlim/cc-top/internal/burnrate"
	"github.com/nixlim/cc-top/internal/config"
)

func TestFormatNumber(t *testing.T) {
	tests := []struct {
		input int64
		want  string
	}{
		{0, "0"},
		{1, "1"},
		{999, "999"},
		{1000, "1,000"},
		{1234, "1,234"},
		{12345, "12,345"},
		{123456, "123,456"},
		{1234567, "1,234,567"},
		{-1234, "-1,234"},
	}

	for _, tt := range tests {
		got := formatNumber(tt.input)
		if got != tt.want {
			t.Errorf("formatNumber(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestTrendArrow(t *testing.T) {
	tests := []struct {
		trend burnrate.TrendDirection
		want  string
	}{
		{burnrate.TrendUp, "^"},
		{burnrate.TrendDown, "v"},
		{burnrate.TrendFlat, "-"},
	}

	for _, tt := range tests {
		got := trendArrow(tt.trend)
		if got != tt.want {
			t.Errorf("trendArrow(%v) = %q, want %q", tt.trend, got, tt.want)
		}
	}
}

func TestGetRateColor(t *testing.T) {
	cfg := config.DefaultConfig()
	m := NewModel(cfg)

	tests := []struct {
		rate float64
		want burnrate.RateColor
	}{
		{0.0, burnrate.ColorGreen},
		{0.25, burnrate.ColorGreen},
		{0.49, burnrate.ColorGreen},
		{0.50, burnrate.ColorYellow},
		{1.00, burnrate.ColorYellow},
		{1.99, burnrate.ColorYellow},
		{2.00, burnrate.ColorRed},
		{5.00, burnrate.ColorRed},
		{100.0, burnrate.ColorRed},
	}

	for _, tt := range tests {
		got := m.getRateColor(tt.rate)
		if got != tt.want {
			t.Errorf("getRateColor(%.2f) = %v, want %v", tt.rate, got, tt.want)
		}
	}
}

func TestGetRateColor_CustomThresholds(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Display.CostColorGreenBelow = 1.00
	cfg.Display.CostColorYellowBelow = 5.00

	m := NewModel(cfg)

	tests := []struct {
		rate float64
		want burnrate.RateColor
	}{
		{0.50, burnrate.ColorGreen},
		{0.99, burnrate.ColorGreen},
		{1.00, burnrate.ColorYellow},
		{4.99, burnrate.ColorYellow},
		{5.00, burnrate.ColorRed},
	}

	for _, tt := range tests {
		got := m.getRateColor(tt.rate)
		if got != tt.want {
			t.Errorf("custom getRateColor(%.2f) = %v, want %v", tt.rate, got, tt.want)
		}
	}
}

func TestRenderBurnRatePanel(t *testing.T) {
	cfg := config.DefaultConfig()
	mockBR := &mockBurnRateProvider{
		global: burnrate.BurnRate{
			TotalCost:     42.50,
			HourlyRate:    3.00,
			Trend:         burnrate.TrendUp,
			TokenVelocity: 15000,
		},
	}

	m := NewModel(cfg, WithBurnRateProvider(mockBR))
	m.width = 120
	m.height = 40
	m.cachedBurnRate = m.computeBurnRate()

	panel := m.renderBurnRatePanel(60, 10)
	stripped := stripAnsi(panel)
	if panel == "" {
		t.Error("renderBurnRatePanel returned empty string")
	}
	if !strings.Contains(stripped, "Burn Rate") {
		t.Error("panel should contain title 'Burn Rate'")
	}
	if !strings.Contains(stripped, "Cost (all sessions):") {
		t.Error("panel should contain 'Cost (all sessions):' label")
	}
	if !strings.Contains(stripped, "$42.50") {
		t.Error("panel should contain total cost $42.50")
	}
	if !strings.Contains(stripped, "Rate (hourly):") {
		t.Error("panel should contain 'Rate (hourly):' label")
	}
	if !strings.Contains(stripped, "Projected Spend:") {
		t.Error("panel should contain 'Projected Spend:' label")
	}
}

func TestRenderBurnRatePanel_NilProvider(t *testing.T) {
	cfg := config.DefaultConfig()
	m := NewModel(cfg)
	m.width = 120
	m.height = 40

	// Should not panic with nil provider.
	panel := m.renderBurnRatePanel(60, 10)
	stripped := stripAnsi(panel)
	if panel == "" {
		t.Error("renderBurnRatePanel returned empty string with nil provider")
	}
	if !strings.Contains(stripped, "Cost (all sessions): $0.00") {
		t.Error("burn rate panel with nil provider should show 'Cost (all sessions): $0.00'")
	}
}

func TestShortModel(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"claude-opus-4-6", "opus-4-6"},
		{"claude-sonnet-4-5-20250929", "sonnet-4-5"},
		{"claude-haiku-4-5-20251001", "haiku-4-5"},
		{"unknown", "unknown"},
		{"", ""},
		{"claude-", "claude-"},
		{"custom-model", "custom-model"},
	}

	for _, tt := range tests {
		got := shortModel(tt.input)
		if got != tt.want {
			t.Errorf("shortModel(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestRenderBurnRatePanel_WithProjections(t *testing.T) {
	cfg := config.DefaultConfig()
	mockBR := &mockBurnRateProvider{
		global: burnrate.BurnRate{
			TotalCost:         10.00,
			HourlyRate:        1.00,
			Trend:             burnrate.TrendFlat,
			TokenVelocity:     5000,
			DailyProjection:   24.00,
			MonthlyProjection: 720.00,
		},
	}

	m := NewModel(cfg, WithBurnRateProvider(mockBR))
	m.width = 120
	m.height = 40
	m.cachedBurnRate = m.computeBurnRate()

	panel := m.renderBurnRatePanel(60, 14)
	stripped := stripAnsi(panel)

	if !strings.Contains(stripped, "$24.00/day") {
		t.Error("panel should contain daily projection")
	}
	if !strings.Contains(stripped, "$720.00/mon") {
		t.Error("panel should contain monthly projection")
	}
}

func TestRenderBurnRatePanel_WithPerModel(t *testing.T) {
	cfg := config.DefaultConfig()
	mockBR := &mockBurnRateProvider{
		global: burnrate.BurnRate{
			TotalCost:     20.00,
			HourlyRate:    4.00,
			Trend:         burnrate.TrendUp,
			TokenVelocity: 10000,
			PerModel: []burnrate.ModelBurnRate{
				{Model: "claude-opus-4-6", HourlyRate: 3.00, TotalCost: 15.00},
				{Model: "claude-sonnet-4-5-20250929", HourlyRate: 1.00, TotalCost: 5.00},
			},
			DailyProjection:   96.00,
			MonthlyProjection: 2880.00,
		},
	}

	m := NewModel(cfg, WithBurnRateProvider(mockBR))
	m.width = 120
	m.height = 40
	m.cachedBurnRate = m.computeBurnRate()

	panel := m.renderBurnRatePanel(60, 16)
	stripped := stripAnsi(panel)

	if !strings.Contains(stripped, "opus-4-6") {
		t.Error("panel should contain shortened opus model name")
	}
	if !strings.Contains(stripped, "sonnet-4-5") {
		t.Error("panel should contain shortened sonnet model name")
	}
}

func TestRenderBurnRatePanel_SingleModelNoBreakdown(t *testing.T) {
	cfg := config.DefaultConfig()
	mockBR := &mockBurnRateProvider{
		global: burnrate.BurnRate{
			TotalCost:  5.00,
			HourlyRate: 1.00,
			PerModel: []burnrate.ModelBurnRate{
				{Model: "claude-opus-4-6", HourlyRate: 1.00, TotalCost: 5.00},
			},
		},
	}

	m := NewModel(cfg, WithBurnRateProvider(mockBR))
	m.width = 120
	m.height = 40
	m.cachedBurnRate = m.computeBurnRate()

	panel := m.renderBurnRatePanel(60, 14)
	stripped := stripAnsi(panel)

	// Single model should NOT show per-model breakdown.
	if strings.Contains(stripped, "opus-4-6") {
		t.Error("single model should not show per-model breakdown")
	}
}

func TestRenderBurnRatePanel_SessionAware(t *testing.T) {
	cfg := config.DefaultConfig()
	mockBR := &mockBurnRateProvider{
		global: burnrate.BurnRate{TotalCost: 10.00},
		perSess: map[string]burnrate.BurnRate{
			"sess-001": {TotalCost: 5.00, HourlyRate: 1.00},
		},
	}

	m := NewModel(cfg, WithBurnRateProvider(mockBR))
	m.width = 120
	m.height = 40

	// Global view: computeBurnRate populates the cache, getBurnRate reads it.
	m.cachedBurnRate = m.computeBurnRate()
	br := m.getBurnRate()
	if br.TotalCost != 10.00 {
		t.Errorf("global burn rate TotalCost = %.2f, want 10.00", br.TotalCost)
	}

	// Global view panel should say "all sessions".
	panel := m.renderBurnRatePanel(60, 10)
	stripped := stripAnsi(panel)
	if !strings.Contains(stripped, "Cost (all sessions):") {
		t.Error("global panel should contain 'Cost (all sessions):' label")
	}

	// Session-specific view.
	m.selectedSession = "sess-001"
	m.cachedBurnRate = m.computeBurnRate()
	br = m.getBurnRate()
	if br.TotalCost != 5.00 {
		t.Errorf("session burn rate TotalCost = %.2f, want 5.00", br.TotalCost)
	}

	// Session view panel should say "session".
	panel = m.renderBurnRatePanel(60, 10)
	stripped = stripAnsi(panel)
	if !strings.Contains(stripped, "Cost (session):") {
		t.Error("session panel should contain 'Cost (session):' label")
	}
}
