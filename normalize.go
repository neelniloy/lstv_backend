package main

import (
	"regexp"
	"strings"
)

// NormalizeName removes quality indicators and other tags from the channel name.
func NormalizeName(name string) string {
	normalized := name

	// 0. Unify separators (hyphens/underscores to spaces)
	normalized = strings.ReplaceAll(normalized, "-", " ")
	normalized = strings.ReplaceAll(normalized, "_", " ")

	// 1. Strip leading digits followed by space (only if 3+ digits, e.g., "101 NBC" -> "NBC")
	// This preserves "5 Minute Craft" but removes indexing numbers.
	normalized = regexp.MustCompile(`^\d{3,}\s+`).ReplaceAllString(normalized, "")

	// 2. Standardize ampersands (e.g., "&Pictures" or "& Pictures" -> "& Pictures")
	normalized = regexp.MustCompile(`\s*&\s*`).ReplaceAllString(normalized, " & ")

	// 3. Tags to remove (regex)
	tags := []string{
		`(?i)\bHD\b`,
		`(?i)\bFHD\b`,
		`(?i)\bSD\b`,
		`(?i)\bUHD\b`,
		`(?i)\b4K\b`,
		`(?i)\b\d+p\b`, // e.g., 1080p, 720p, 480p
		`(?i)\b\d+i\b`, // e.g., 576i, 1080i
		`(?i)\bLIVE\b`,
		`(?i)\bBackup\b`,
		`(?i)\[.*\]`, // Remove anything in brackets
		`(?i)\(.*\s*quality\s*\)`,
		`(?i)\(.*\bUSA\b.*\)`,
		`(?i)\(.*\bHindi\b.*\)`,
		`(?i)\(.*\bUK\b.*\)`,
		`(?i)\(.*\bAU\b.*\)`,
	}

	for _, tag := range tags {
		re := regexp.MustCompile(tag)
		normalized = re.ReplaceAllString(normalized, "")
	}

	// 4. Unify common word variations (plurals/singulars)
	wordReplacements := map[string]string{
		"Minutes": "Minute",
		"Crafts":  "Craft",
	}
	for old, new := range wordReplacements {
		re := regexp.MustCompile(`(?i)\b` + old + `\b`)
		normalized = re.ReplaceAllString(normalized, new)
	}

	// 5. Remove empty parentheses () and any residual double spaces
	normalized = regexp.MustCompile(`\(\s*\)`).ReplaceAllString(normalized, "")

	// Clean up extra spaces
	reSpaces := regexp.MustCompile(`\s+`)
	normalized = reSpaces.ReplaceAllString(normalized, " ")
	normalized = strings.TrimSpace(normalized)

	// Final touch: If it's all lowercase, title-case it
	if normalized == strings.ToLower(normalized) && len(normalized) > 0 {
		normalized = strings.Title(normalized)
	}

	return normalized
}

// NormalizeCategory standardizes category names and filters out provider tags.
func NormalizeCategory(category string, channelName string) string {
	// Handle multiple categories (e.g., "News;Sports")
	parts := strings.Split(category, ";")
	if len(parts) > 1 {
		category = parts[0]
	}

	c := strings.ToLower(strings.TrimSpace(category))
	n := strings.ToLower(channelName)

	// Provider tags to discard
	providers := []string{"lgtv", "samsung", "plex", "yupp", "fast", "hilay", "ott", "playlist"}
	for _, p := range providers {
		if strings.Contains(c, p) {
			c = "" // Mark for reset to General
			break
		}
	}

	if c == "" || c == "undefined" || c == "general" || c == "other" {
		c = "General"
	}

	// Infer from Name: If it's General, try to find a better category in the Name
	if c == "General" {
		if strings.Contains(n, "news") {
			return "News"
		}
		if strings.Contains(n, "sport") {
			return "Sports"
		}
		if strings.Contains(n, "movie") || strings.Contains(n, "cinema") || strings.Contains(n, "film") {
			return "Movies"
		}
		if strings.Contains(n, "music") {
			return "Music"
		}
		if strings.Contains(n, "kids") || strings.Contains(n, "children") {
			return "Kids"
		}
	}

	mappings := map[string]string{
		"movie":         "Movies",
		"movies":        "Movies",
		"film":          "Movies",
		"cinema":        "Movies",
		"entertainment": "Entertainment",
		"ent":           "Entertainment",
		"kids":          "Kids",
		"children":      "Kids",
		"sports":        "Sports",
		"sport":         "Sports",
		"news":          "News",
		"music":         "Music",
		"religious":     "Religious",
		"religion":      "Religious",
		"comedy":        "Comedy",
		"business":      "Business",
	}

	// Check for direct match or partial match
	for key, val := range mappings {
		if strings.Contains(c, key) {
			return val
		}
	}

	// Capitalize first letter of unknown categories
	if len(c) > 0 && c != "General" {
		return strings.ToUpper(c[:1]) + c[1:]
	}
	return "General"
}
