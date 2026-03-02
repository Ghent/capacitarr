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

		matched := matchesRule(item, rule)
		slog.Debug("Rule evaluation", "component", "engine",
			"title", item.Title, "rule", fmt.Sprintf("%s %s %s", rule.Field, rule.Operator, rule.Value),
			"matched", matched)
		if matched {
			ruleName := fmt.Sprintf("%s %s %s", rule.Field, rule.Operator, rule.Value)

			// Use the new effect field if set; fall back to legacy type+intensity
			effect := rule.Effect
			if effect == "" {
				effect = legacyEffect(rule.Type, rule.Intensity)
			}

			switch effect {
			case "always_keep":
				// Immune to deletion — absolute override
				factor := ScoreFactor{
					Name:         fmt.Sprintf("Always keep: %s", ruleName),
					RawScore:     0.0,
					Weight:       0,
					Contribution: 0.0,
					Type:         "rule",
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
				})

			case "lean_remove":
				modifier *= 1.2
				reasons = append(reasons, fmt.Sprintf("Lean toward removing (%s %s)", rule.Field, rule.Value))
				ruleFactors = append(ruleFactors, ScoreFactor{
					Name:         fmt.Sprintf("Lean remove: %s", ruleName),
					RawScore:     1.0,
					Weight:       0,
					Contribution: 0.0,
					Type:         "rule",
				})

			case "prefer_remove":
				modifier *= 2.0
				reasons = append(reasons, fmt.Sprintf("Prefer to remove (%s %s)", rule.Field, rule.Value))
				ruleFactors = append(ruleFactors, ScoreFactor{
					Name:         fmt.Sprintf("Prefer remove: %s", ruleName),
					RawScore:     1.0,
					Weight:       0,
					Contribution: 0.0,
					Type:         "rule",
				})

			case "always_remove":
				modifier *= 100.0 // Ensure it hits the ceiling
				reasons = append(reasons, fmt.Sprintf("Always remove (%s %s)", rule.Field, rule.Value))
				ruleFactors = append(ruleFactors, ScoreFactor{
					Name:         fmt.Sprintf("Always remove: %s", ruleName),
					RawScore:     1.0,
					Weight:       0,
					Contribution: 0.0,
					Type:         "rule",
				})
			}
		}
	}
	return false, modifier, strings.Join(reasons, ", "), ruleFactors
}

// legacyEffect converts old type+intensity fields to the new effect value.
// Used for backward compatibility with rules that haven't been migrated.
func legacyEffect(ruleType, intensity string) string {
	switch {
	case ruleType == "protect" && intensity == "absolute":
		return "always_keep"
	case ruleType == "protect" && intensity == "strong":
		return "prefer_keep"
	case ruleType == "protect":
		return "lean_keep"
	case ruleType == "target" && intensity == "absolute":
		return "always_remove"
	case ruleType == "target" && intensity == "strong":
		return "prefer_remove"
	case ruleType == "target":
		return "lean_remove"
	default:
		return "lean_keep"
	}
}

func matchesRule(item integrations.MediaItem, rule db.CustomRule) bool {
	prop := strings.ToLower(rule.Field)
	cond := strings.ToLower(rule.Operator)
	val := strings.ToLower(rule.Value)

	switch prop {
	case "title":
		return stringMatch(strings.ToLower(item.Title), cond, val)
	case "quality":
		return stringMatch(strings.ToLower(item.QualityProfile), cond, val)
	case "seriesstatus":
		// Match against status (e.g., Ended, Continuing)
		return stringMatch(strings.ToLower(item.SeriesStatus), cond, val)
	case "tag":
		for _, tag := range item.Tags {
			if stringMatch(strings.ToLower(tag), cond, val) {
				return true
			}
		}
		return false
	case "genre":
		return stringMatch(strings.ToLower(item.Genre), cond, val)
	case "rating":
		ruleNum, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return false
		}
		return numberMatch(item.Rating, cond, ruleNum)
	case "sizebytes":
		ruleNum, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return false
		}
		return numberMatch(float64(item.SizeBytes), cond, ruleNum)
	case "timeinlibrary":
		if item.AddedAt == nil || item.AddedAt.IsZero() {
			return false
		}
		ruleNum, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return false
		}
		days := time.Since(*item.AddedAt).Hours() / 24.0
		return numberMatch(days, cond, ruleNum)
	case "seasoncount":
		ruleNum, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return false
		}
		return numberMatch(float64(item.SeasonNumber), cond, ruleNum)
	case "episodecount":
		ruleNum, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return false
		}
		return numberMatch(float64(item.EpisodeCount), cond, ruleNum)
	case "monitored":
		expected := val == "true"
		return item.Monitored == expected
	case "playcount":
		ruleNum, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return false
		}
		return numberMatch(float64(item.PlayCount), cond, ruleNum)
	case "requested":
		expected := val == "true"
		return item.IsRequested == expected
	case "requestcount":
		ruleNum, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return false
		}
		return numberMatch(float64(item.RequestCount), cond, ruleNum)
	case "language":
		return stringMatch(strings.ToLower(item.Language), cond, val)
	case "type":
		return stringMatch(strings.ToLower(string(item.Type)), cond, val)
	case "year":
		ruleNum, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return false
		}
		return numberMatch(float64(item.Year), cond, ruleNum)
	}

	return false
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
