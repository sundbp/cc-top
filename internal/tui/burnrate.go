package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/nixlim/cc-top/internal/burnrate"
)

// renderBurnRatePanel renders the burn rate odometer panel showing total cost,
// hourly rate, trend, and token velocity.
func (m Model) renderBurnRatePanel(w, h int) string {
	br := m.getBurnRate()

	// Determine color based on hourly rate and configurable thresholds.
	rateColor := m.getRateColor(br.HourlyRate)
	colorStyle := m.colorStyleForRate(rateColor)

	// Content height inside borders (border takes 2 lines).
	contentH := h - 2
	if contentH < 1 {
		contentH = 1
	}

	// Content width inside borders.
	contentW := w - 4
	if contentW < 10 {
		contentW = 10
	}

	// Build the content.
	var lines []string

	// Title line.
	lines = append(lines, panelTitleStyle.Render("Burn Rate"))

	// Cost label adapts to whether we're viewing a single session or global.
	costLabel := "Cost (all sessions):"
	if m.selectedSession != "" {
		costLabel = "Cost (session):"
	}
	costLine := fmt.Sprintf("%s $%.2f", costLabel, br.TotalCost)
	lines = append(lines, costGreenStyle.Render(costLine))

	// Hourly rate and trend.
	trendArrow := trendArrow(br.Trend)
	rateLine := fmt.Sprintf("Rate (hourly): $%.2f/hr %s", br.HourlyRate, trendArrow)
	lines = append(lines, colorStyle.Render(rateLine))

	// Token velocity.
	tokenLine := fmt.Sprintf("%s tokens/min", formatNumber(int64(br.TokenVelocity)))
	lines = append(lines, dimStyle.Render(tokenLine))

	// Cost projections.
	projLine := fmt.Sprintf("Projected Spend: $%.2f/day  $%.2f/mon", br.DailyProjection, br.MonthlyProjection)
	lines = append(lines, dimStyle.Render(projLine))

	// Per-model cost breakdown (shown when multiple models are present).
	if len(br.PerModel) > 1 {
		shown := br.PerModel
		if len(shown) > 3 {
			shown = shown[:3]
		}
		for _, pm := range shown {
			modelLine := fmt.Sprintf("  %s $%.2f/hr $%.2f", shortModel(pm.Model), pm.HourlyRate, pm.TotalCost)
			lines = append(lines, dimStyle.Render(modelLine))
		}
	}

	content := strings.Join(lines, "\n")

	// Wrap in panel border, clamping content to fit.
	return renderBorderedPanel(content, w, h)
}

// getBurnRate returns the cached burn rate (updated on tick, not every render).
func (m Model) getBurnRate() burnrate.BurnRate {
	return m.cachedBurnRate
}

// computeBurnRate retrieves a fresh burn rate from the provider.
// Called only from the tick handler to avoid per-render recalculation.
func (m Model) computeBurnRate() burnrate.BurnRate {
	if m.burnRate == nil {
		return burnrate.BurnRate{}
	}
	if m.selectedSession != "" {
		return m.burnRate.Get(m.selectedSession)
	}
	return m.burnRate.GetGlobal()
}

// getRateColor returns the color classification for a given hourly rate.
func (m Model) getRateColor(hourlyRate float64) burnrate.RateColor {
	if hourlyRate < m.cfg.Display.CostColorGreenBelow {
		return burnrate.ColorGreen
	}
	if hourlyRate < m.cfg.Display.CostColorYellowBelow {
		return burnrate.ColorYellow
	}
	return burnrate.ColorRed
}

// colorStyleForRate returns the lipgloss style for a given rate color.
func (m Model) colorStyleForRate(rc burnrate.RateColor) lipgloss.Style {
	switch rc {
	case burnrate.ColorGreen:
		return costGreenStyle
	case burnrate.ColorYellow:
		return costYellowStyle
	case burnrate.ColorRed:
		return costRedStyle
	default:
		return costGreenStyle
	}
}

// trendArrow returns the unicode arrow for a trend direction.
func trendArrow(t burnrate.TrendDirection) string {
	switch t {
	case burnrate.TrendUp:
		return "^"
	case burnrate.TrendDown:
		return "v"
	default:
		return "-"
	}
}

// shortModel returns a shortened model name for compact display.
// e.g. "claude-sonnet-4-5-20250929" -> "sonnet-4-5", "claude-opus-4-6" -> "opus-4-6".
func shortModel(model string) string {
	// Strip "claude-" prefix.
	s := strings.TrimPrefix(model, "claude-")
	// Strip date suffix (e.g. "-20250929").
	if len(s) > 9 && s[len(s)-9] == '-' {
		candidate := s[len(s)-8:]
		allDigits := true
		for _, c := range candidate {
			if c < '0' || c > '9' {
				allDigits = false
				break
			}
		}
		if allDigits {
			s = s[:len(s)-9]
		}
	}
	if s == "" {
		return model
	}
	return s
}

// formatNumber formats an int64 with comma separators (e.g., 1,234,567).
func formatNumber(n int64) string {
	if n < 0 {
		return "-" + formatNumber(-n)
	}

	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}

	var result strings.Builder
	remainder := len(s) % 3
	if remainder > 0 {
		result.WriteString(s[:remainder])
	}
	for i := remainder; i < len(s); i += 3 {
		if result.Len() > 0 {
			result.WriteByte(',')
		}
		result.WriteString(s[i : i+3])
	}
	return result.String()
}
