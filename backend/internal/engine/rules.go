// Package engine implements the capacity management scoring and rule evaluation logic.
package engine

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"capacitarr/internal/db"
	"capacitarr/internal/integrations"
)

const boolTrue = "true"

// applyRules checks if a media item meets any custom rules and applies score modifiers.
// Uses the new "effect" field for the combined keep/remove spectrum.
// Implements "Keep Always Wins" conflict resolution:
//   - If any matching rule has "always_keep", the item is immune (absolute override).
//   - All other effects multiply together.
//
// Returns (isAbsolutelyProtected, scoreModifier, reasonString, ruleFactors)
func applyRules(item integrations.MediaItem, rules []db.CustomRule) (bool, float64, string, []ScoreFactor) {
	var reasons []string
	var ruleFactors []ScoreFactor
	modifier := 1.0

	for _, rule := range rules {
		// Skip disabled rules
		if !rule.Enabled {
			continue
		}
		// Skip rules scoped to a different integration
		if rule.IntegrationID != nil && *rule.IntegrationID != item.IntegrationID {
			continue
		}

		matched, matchedValue := matchesRuleWithValue(item, rule)
		slog.Debug("Rule evaluation", "component", "engine",
			"title", item.Title, "rule", fmt.Sprintf("%s %s %s", rule.Field, rule.Operator, rule.Value),
			"matched", matched, "matchedValue", matchedValue)
		if matched {
			ruleName := fmt.Sprintf("%s %s %s", rule.Field, rule.Operator, rule.Value)

			effect := rule.Effect

			switch effect {
			case "always_keep":
				// Immune to deletion — absolute override
				factor := ScoreFactor{
					Name:         fmt.Sprintf("Always keep: %s", ruleName),
					RawScore:     0.0,
					Weight:       0,
					Contribution: 0.0,
					Type:         "rule",
					MatchedValue: matchedValue,
				}
				return true, 0.0, fmt.Sprintf("Always keep: %s", ruleName), []ScoreFactor{factor}

			case "prefer_keep":
				modifier *= 0.2
				reasons = append(reasons, fmt.Sprintf("Prefer to keep (%s %s)", rule.Field, rule.Value))
				ruleFactors = append(ruleFactors, ScoreFactor{
					Name:         fmt.Sprintf("Prefer keep: %s", ruleName),
					RawScore:     0.2,
					Weight:       0,
					Contribution: 0.0,
					Type:         "rule",
					MatchedValue: matchedValue,
				})

			case "lean_keep":
				modifier *= 0.5
				reasons = append(reasons, fmt.Sprintf("Lean toward keeping (%s %s)", rule.Field, rule.Value))
				ruleFactors = append(ruleFactors, ScoreFactor{
					Name:         fmt.Sprintf("Lean keep: %s", ruleName),
					RawScore:     0.5,
					Weight:       0,
					Contribution: 0.0,
					Type:         "rule",
					MatchedValue: matchedValue,
				})

			case "lean_remove":
				modifier *= 1.5
				reasons = append(reasons, fmt.Sprintf("Lean toward removing (%s %s)", rule.Field, rule.Value))
				ruleFactors = append(ruleFactors, ScoreFactor{
					Name:         fmt.Sprintf("Lean remove: %s", ruleName),
					RawScore:     1.5,
					Weight:       0,
					Contribution: 0.0,
					Type:         "rule",
					MatchedValue: matchedValue,
				})

			case "prefer_remove":
				modifier *= 3.0
				reasons = append(reasons, fmt.Sprintf("Prefer to remove (%s %s)", rule.Field, rule.Value))
				ruleFactors = append(ruleFactors, ScoreFactor{
					Name:         fmt.Sprintf("Prefer remove: %s", ruleName),
					RawScore:     3.0,
					Weight:       0,
					Contribution: 0.0,
					Type:         "rule",
					MatchedValue: matchedValue,
				})

			case "always_remove":
				modifier *= 100.0 // Ensure it hits the ceiling
				reasons = append(reasons, fmt.Sprintf("Always remove (%s %s)", rule.Field, rule.Value))
				ruleFactors = append(ruleFactors, ScoreFactor{
					Name:         fmt.Sprintf("Always remove: %s", ruleName),
					RawScore:     100.0,
					Weight:       0,
					Contribution: 0.0,
					Type:         "rule",
					MatchedValue: matchedValue,
				})
			}
		}
	}
	return false, modifier, strings.Join(reasons, ", "), ruleFactors
}



// matchesRuleWithValue checks if a media item matches a rule and returns the actual
// item value that triggered (or confirmed) the match. For positive matches on array
// fields (tags), returns the specific element that matched. For negation operators,
// returns the actual value(s) to show why the rule triggered.
func matchesRuleWithValue(item integrations.MediaItem, rule db.CustomRule) (bool, string) {
	prop := strings.ToLower(rule.Field)
	cond := strings.ToLower(rule.Operator)
	val := strings.ToLower(rule.Value)

	switch prop {
	case "title":
		matched := stringMatch(strings.ToLower(item.Title), cond, val)
		return matched, item.Title
	case "quality":
		matched := stringMatch(strings.ToLower(item.QualityProfile), cond, val)
		return matched, item.QualityProfile
	case "seriesstatus":
		matched := stringMatch(strings.ToLower(item.SeriesStatus), cond, val)
		return matched, item.SeriesStatus
	case "tag":
		// For positive operators (==, contains), find the specific tag that matched
		if cond == "==" || cond == "contains" {
			for _, tag := range item.Tags {
				if stringMatch(strings.ToLower(tag), cond, val) {
					return true, tag
				}
			}
			return false, strings.Join(item.Tags, ", ")
		}
		// For negation operators (!=, !contains), check all tags
		for _, tag := range item.Tags {
			if !stringMatchNegated(strings.ToLower(tag), cond, val) {
				return false, tag // This tag violated the negation
			}
		}
		// All tags passed the negation check (or no tags exist)
		if len(item.Tags) == 0 {
			return true, "(no tags)"
		}
		return true, strings.Join(item.Tags, ", ")
	case "genre":
		matched := stringMatch(strings.ToLower(item.Genre), cond, val)
		return matched, item.Genre
	case "rating":
		ruleNum, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return false, ""
		}
		matched := numberMatch(item.Rating, cond, ruleNum)
		return matched, fmt.Sprintf("%.1f", item.Rating)
	case "sizebytes":
		ruleNum, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return false, ""
		}
		matched := numberMatch(float64(item.SizeBytes), cond, ruleNum)
		gb := float64(item.SizeBytes) / (1024 * 1024 * 1024)
		return matched, fmt.Sprintf("%.2f GB", gb)
	case "timeinlibrary":
		if item.AddedAt == nil || item.AddedAt.IsZero() {
			return false, ""
		}
		ruleNum, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return false, ""
		}
		days := time.Since(*item.AddedAt).Hours() / 24.0
		switch cond {
		case "in_last":
			return days <= ruleNum, fmt.Sprintf("%.0f days", days)
		case "over_ago":
			return days > ruleNum, fmt.Sprintf("%.0f days", days)
		default:
			matched := numberMatch(days, cond, ruleNum)
			return matched, fmt.Sprintf("%.0f days", days)
		}
	case "seasoncount":
		ruleNum, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return false, ""
		}
		matched := numberMatch(float64(item.SeasonNumber), cond, ruleNum)
		return matched, fmt.Sprintf("%d", item.SeasonNumber)
	case "episodecount":
		ruleNum, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return false, ""
		}
		matched := numberMatch(float64(item.EpisodeCount), cond, ruleNum)
		return matched, fmt.Sprintf("%d", item.EpisodeCount)
	case "monitored":
		expected := val == boolTrue
		matched := item.Monitored == expected
		return matched, fmt.Sprintf("%v", item.Monitored)
	case "playcount":
		ruleNum, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return false, ""
		}
		matched := numberMatch(float64(item.PlayCount), cond, ruleNum)
		return matched, fmt.Sprintf("%d", item.PlayCount)
	case "requested":
		expected := val == boolTrue
		matched := item.IsRequested == expected
		return matched, fmt.Sprintf("%v", item.IsRequested)
	case "requestcount":
		ruleNum, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return false, ""
		}
		matched := numberMatch(float64(item.RequestCount), cond, ruleNum)
		return matched, fmt.Sprintf("%d", item.RequestCount)
	case "language":
		matched := stringMatch(strings.ToLower(item.Language), cond, val)
		return matched, item.Language
	case "type":
		matched := stringMatch(strings.ToLower(string(item.Type)), cond, val)
		return matched, string(item.Type)
	case "year":
		ruleNum, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return false, ""
		}
		matched := numberMatch(float64(item.Year), cond, ruleNum)
		return matched, fmt.Sprintf("%d", item.Year)
	case "lastplayed":
		if item.LastPlayed == nil || item.LastPlayed.IsZero() {
			if cond == "never" {
				return true, "never played"
			}
			if cond == "over_ago" {
				return true, "never played (counts as over)"
			}
			return false, "never played"
		}
		if cond == "never" {
			return false, fmt.Sprintf("%.0f days ago", time.Since(*item.LastPlayed).Hours()/24.0)
		}
		ruleNum, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return false, ""
		}
		days := time.Since(*item.LastPlayed).Hours() / 24.0
		switch cond {
		case "in_last":
			return days <= ruleNum, fmt.Sprintf("%.0f days ago", days)
		case "over_ago":
			return days > ruleNum, fmt.Sprintf("%.0f days ago", days)
		}
		return false, ""
	case "requestedby":
		actual := item.RequestedBy
		switch cond {
		case "==":
			return strings.EqualFold(actual, val), actual
		case "!=":
			return !strings.EqualFold(actual, val), actual
		case "contains":
			return strings.Contains(strings.ToLower(actual), strings.ToLower(val)), actual
		case "!contains":
			return !strings.Contains(strings.ToLower(actual), strings.ToLower(val)), actual
		}
		return false, actual
	case "incollection":
		inCollection := len(item.Collections) > 0
		ruleBool := val == boolTrue
		return inCollection == ruleBool, fmt.Sprintf("%d collections", len(item.Collections))
	case "watchedbyreq":
		ruleBool := val == boolTrue
		return item.WatchedByRequestor == ruleBool, fmt.Sprintf("watched by requestor: %v", item.WatchedByRequestor)
	}

	return false, ""
}

// stringMatchNegated returns true if the actual value passes the negation check.
// Used for array fields where we need to check each element individually.
func stringMatchNegated(actual, cond, expected string) bool {
	switch cond {
	case "!=":
		return actual != expected
	case "!contains":
		return !strings.Contains(actual, expected)
	}
	return true
}

func stringMatch(actual, cond, expected string) bool {
	switch cond {
	case "==":
		return actual == expected
	case "!=":
		return actual != expected
	case "contains":
		return strings.Contains(actual, expected)
	case "!contains":
		return !strings.Contains(actual, expected)
	}
	return false
}

func numberMatch(actual float64, cond string, expected float64) bool {
	switch cond {
	case "==":
		return actual == expected
	case "!=":
		return actual != expected
	case ">":
		return actual > expected
	case ">=":
		return actual >= expected
	case "<":
		return actual < expected
	case "<=":
		return actual <= expected
	}
	return false
}
