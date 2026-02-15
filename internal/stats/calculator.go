// Package stats provides aggregate statistics computation from the
// in-memory state store data. All functions are pure computations
// with no side effects.
package stats

import (
	"sort"
	"strconv"
	"strings"

	"github.com/nixlim/cc-top/internal/state"
)

// Calculator computes aggregate statistics from state store data.
type Calculator struct{}

// NewCalculator creates a new Calculator instance.
func NewCalculator() *Calculator {
	return &Calculator{}
}

// Compute calculates the full DashboardStats from the given sessions.
// This is a pure function: it reads from the session data and produces
// computed statistics with no side effects.
func (c *Calculator) Compute(sessions []state.SessionData) DashboardStats {
	stats := DashboardStats{
		ToolAcceptance: make(map[string]float64),
	}

	stats.LinesAdded, stats.LinesRemoved = c.computeLinesOfCode(sessions)
	stats.Commits = c.computeCounterMetric(sessions, "claude_code.commit.count")
	stats.PRs = c.computeCounterMetric(sessions, "claude_code.pull_request.count")
	stats.ToolAcceptance = c.computeToolAcceptance(sessions)
	stats.CacheEfficiency = c.computeCacheEfficiency(sessions)
	stats.AvgAPILatency = c.computeAvgAPILatency(sessions)
	stats.ModelBreakdown = c.computeModelBreakdown(sessions)
	stats.TopTools = c.computeTopTools(sessions)
	stats.ErrorRate = c.computeErrorRate(sessions)

	return stats
}

// computeLinesOfCode returns the total lines added and removed across
// all sessions based on claude_code.lines_of_code.count metrics.
// For cumulative counters, only the latest (last in append-ordered list)
// value per session+type is used.
func (c *Calculator) computeLinesOfCode(sessions []state.SessionData) (added, removed int) {
	for i := range sessions {
		var lastAdded, lastRemoved float64
		for _, m := range sessions[i].Metrics {
			if m.Name != "claude_code.lines_of_code.count" {
				continue
			}
			switch m.Attributes["type"] {
			case "added":
				lastAdded = m.Value
			case "removed":
				lastRemoved = m.Value
			}
		}
		added += int(lastAdded)
		removed += int(lastRemoved)
	}
	return
}

// computeCounterMetric returns the sum of the latest value of a named
// cumulative counter metric across all sessions. For each session, only
// the last (most recent) metric value matching the name is used.
func (c *Calculator) computeCounterMetric(sessions []state.SessionData, metricName string) int {
	var total int
	for i := range sessions {
		var last float64
		for _, m := range sessions[i].Metrics {
			if m.Name == metricName {
				last = m.Value
			}
		}
		total += int(last)
	}
	return total
}

// computeToolAcceptance calculates the acceptance rate for each tool
// from claude_code.code_edit_tool.decision metrics.
// Rate = accept count / total count per tool.
// For cumulative counters, only the latest value per session+tool+decision
// combination is used.
func (c *Calculator) computeToolAcceptance(sessions []state.SessionData) map[string]float64 {
	type toolCounts struct {
		accepted int
		total    int
	}
	tools := make(map[string]*toolCounts)

	type toolDecisionKey struct {
		tool     string
		decision string
	}

	for i := range sessions {
		latest := make(map[toolDecisionKey]float64)

		for _, m := range sessions[i].Metrics {
			if m.Name != "claude_code.code_edit_tool.decision" {
				continue
			}
			key := toolDecisionKey{
				tool:     m.Attributes["tool"],
				decision: m.Attributes["decision"],
			}
			latest[key] = m.Value
		}

		for key, val := range latest {
			tc, ok := tools[key.tool]
			if !ok {
				tc = &toolCounts{}
				tools[key.tool] = tc
			}
			count := int(val)
			tc.total += count
			if strings.EqualFold(key.decision, "accept") {
				tc.accepted += count
			}
		}
	}

	result := make(map[string]float64, len(tools))
	for name, tc := range tools {
		if tc.total == 0 {
			result[name] = 0
		} else {
			result[name] = float64(tc.accepted) / float64(tc.total)
		}
	}
	return result
}

// computeCacheEfficiency calculates cache efficiency as:
// cacheRead / (input + cacheRead)
// Returns 0 if the denominator is zero (no token data).
// For cumulative counters, only the latest value per session+type is used.
func (c *Calculator) computeCacheEfficiency(sessions []state.SessionData) float64 {
	var cacheRead, input float64

	for i := range sessions {
		var lastCacheRead, lastInput float64
		for _, m := range sessions[i].Metrics {
			if m.Name != "claude_code.token.usage" {
				continue
			}
			switch m.Attributes["type"] {
			case "cacheRead":
				lastCacheRead = m.Value
			case "input":
				lastInput = m.Value
			}
		}
		cacheRead += lastCacheRead
		input += lastInput
	}

	denominator := input + cacheRead
	if denominator == 0 {
		return 0
	}
	return cacheRead / denominator
}

// computeAvgAPILatency calculates the mean duration_ms from api_request
// events, converted to seconds.
func (c *Calculator) computeAvgAPILatency(sessions []state.SessionData) float64 {
	var totalMS float64
	var count int

	for i := range sessions {
		for _, e := range sessions[i].Events {
			if e.Name != "claude_code.api_request" {
				continue
			}
			durStr, ok := e.Attributes["duration_ms"]
			if !ok {
				continue
			}
			dur, err := strconv.ParseFloat(durStr, 64)
			if err != nil {
				continue
			}
			totalMS += dur
			count++
		}
	}

	if count == 0 {
		return 0
	}
	return totalMS / float64(count) / 1000.0 // Convert ms to seconds.
}

// computeModelBreakdown aggregates cost and tokens by model from
// api_request events. Returns sorted by cost descending.
func (c *Calculator) computeModelBreakdown(sessions []state.SessionData) []ModelStats {
	type modelAgg struct {
		cost   float64
		tokens int64
	}
	models := make(map[string]*modelAgg)

	for i := range sessions {
		for _, e := range sessions[i].Events {
			if e.Name != "claude_code.api_request" {
				continue
			}
			model := e.Attributes["model"]
			if model == "" {
				continue
			}

			agg, ok := models[model]
			if !ok {
				agg = &modelAgg{}
				models[model] = agg
			}

			if costStr, ok := e.Attributes["cost_usd"]; ok {
				if cost, err := strconv.ParseFloat(costStr, 64); err == nil {
					agg.cost += cost
				}
			}

			// Sum input and output tokens.
			if inStr, ok := e.Attributes["input_tokens"]; ok {
				if in, err := strconv.ParseInt(inStr, 10, 64); err == nil {
					agg.tokens += in
				}
			}
			if outStr, ok := e.Attributes["output_tokens"]; ok {
				if out, err := strconv.ParseInt(outStr, 10, 64); err == nil {
					agg.tokens += out
				}
			}
		}
	}

	result := make([]ModelStats, 0, len(models))
	for name, agg := range models {
		result = append(result, ModelStats{
			Model:       name,
			TotalCost:   agg.cost,
			TotalTokens: agg.tokens,
		})
	}

	// Sort by cost descending.
	sort.Slice(result, func(i, j int) bool {
		return result[i].TotalCost > result[j].TotalCost
	})
	return result
}

// computeTopTools ranks tools by frequency from tool_result events.
// Returns sorted by count descending.
func (c *Calculator) computeTopTools(sessions []state.SessionData) []ToolUsage {
	tools := make(map[string]int)

	for i := range sessions {
		for _, e := range sessions[i].Events {
			if e.Name != "claude_code.tool_result" {
				continue
			}
			toolName := e.Attributes["tool_name"]
			if toolName != "" {
				tools[toolName]++
			}
		}
	}

	result := make([]ToolUsage, 0, len(tools))
	for name, count := range tools {
		result = append(result, ToolUsage{ToolName: name, Count: count})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Count > result[j].Count
	})
	return result
}

// computeErrorRate calculates the ratio of api_error events to
// api_request events. Returns 0 if there are no api_request events
// (division by zero protection).
func (c *Calculator) computeErrorRate(sessions []state.SessionData) float64 {
	var apiRequests, apiErrors int

	for i := range sessions {
		for _, e := range sessions[i].Events {
			switch e.Name {
			case "claude_code.api_request":
				apiRequests++
			case "claude_code.api_error":
				apiErrors++
			}
		}
	}

	if apiRequests == 0 {
		return 0
	}
	return float64(apiErrors) / float64(apiRequests)
}
