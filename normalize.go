package main

import (
	"regexp"
	"strings"
)

// NormalizeName removes quality indicators and other tags from the channel name.
func NormalizeName(name string) string {
	normalized := name

	// 1. Unify separators (hyphens/underscores/slashes to spaces)
	normalized = strings.ReplaceAll(normalized, "-", " ")
	normalized = strings.ReplaceAll(normalized, "_", " ")
	normalized = strings.ReplaceAll(normalized, "/", " ")

	// 2. Strip leading digits followed by space (only if 3+ digits, e.g., "101 NBC" -> "NBC")
	normalized = regexp.MustCompile(`^\d{3,}\s+`).ReplaceAllString(normalized, "")

	// 3. Standardize ampersands: "&" -> "And" for better semantic grouping
	normalized = regexp.MustCompile(`\s*&\s*`).ReplaceAllString(normalized, " And ")

	// 4. Tags to remove (regex)
	tags := []string{
		`(?i)\bHD\b`,
		`(?i)\bFHD\b`,
		`(?i)\bSD\b`,
		`(?i)\bUHD\b`,
		`(?i)\b4K\b`,
		`(?i)\b\d+p\b`, // e.g., 1080p, 720p, 480p
		`(?i)\b\d+i\b`, // e.g., 576i, 1080i
		`(?i)\bLIVE\b`,
		`(?i)\bBACKUP\b`,
		`(?i)\bOFFICIAL\b`,
		`(?i)\bSTREAM\b`,
		`(?i)\bONLINE\b`,
		`(?i)\[[^\]]*\]`, // Remove anything in brackets (non-greedy)
		`(?i)\([^\)]*\)`, // Remove anything in parentheses (generally noise)
		`(?i)\bENG\b`,
		`(?i)\bITA\b`,
		`(?i)\bESP\b`,
		`(?i)\bFRA\b`,
		`(?i)\bHIN\b`,
		`(?i)\bKOR\b`,
		`(?i)\bJPN\b`,
	}

	for _, tag := range tags {
		re := regexp.MustCompile(tag)
		normalized = re.ReplaceAllString(normalized, "")
	}

	// 5. Unify common word variations
	wordReplacements := map[string]string{
		"Minutes": "Minute",
		"Crafts":  "Craft",
		"&":       "And",
	}
	for old, new := range wordReplacements {
		re := regexp.MustCompile(`(?i)\b` + old + `\b`)
		normalized = re.ReplaceAllString(normalized, new)
	}

	// 6. Clean up extra spaces/symbols
	normalized = regexp.MustCompile(`[^\w\s\-]`).ReplaceAllString(normalized, "") // Keep only alphanumeric, spaces, hyphens
	normalized = regexp.MustCompile(`\s+`).ReplaceAllString(normalized, " ")
	normalized = strings.TrimSpace(normalized)

	// 7. Robust Casing
	if normalized == "" {
		return ""
	}

	// Known acronyms that should always be uppercase
	acronyms := map[string]bool{
		"B4U":    true,
		"MTV":    true,
		"HBO":    true,
		"CNN":    true,
		"BBC":    true,
		"NDTV":   true,
		"CNBC":   true,
		"ESPN":   true,
		"TNT":    true,
		"AMC":    true,
		"AXN":    true,
		"SET":    true,
		"TVP":    true,
		"TLC":    true,
		"SD":     true,
		"UHD":    true,
		"FHD":    true,
		"NGC":    true,
		"STAR":   true,
		"ZEE":    true,
		"ABP":    true,
		"PTC":    true,
		"WB":     true,
		"FX":     true,
		"SYFY":   true,
		"VH1":    true,
		"CW":     true,
		"USA":    true,
		"WBW":    true,
		"SMC":    true,
		"NHK":    true,
		"CGTN":   true,
		"CNA":    true,
		"DW":     true,
		"RT":     true,
		"ALJ":    true,
		"BTV":    true,
		"DBC":    true,
		"ATN":    true,
		"NTV":    true,
		"RTV":    true,
		"SA":     true,
		"9X":     true,
		"SONY":   true,
		"COLOR":  true,
		"COLORS": true,
		"TEN":    true,
		"SPORTS": true, // Sometimes SPORTS is treated as acronym in names like SONY SPORTS
	}

	words := strings.Fields(normalized)
	for i, word := range words {
		upperWord := strings.ToUpper(word)
		if acronyms[upperWord] {
			words[i] = upperWord
		} else {
			// Title case for regular words
			if len(word) > 1 {
				words[i] = strings.ToUpper(word[:1]) + strings.ToLower(word[1:])
			} else {
				words[i] = strings.ToUpper(word)
			}
		}
	}
	normalized = strings.Join(words, " ")

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
	providers := []string{"lgtv", "samsung", "plex", "yupp", "fast", "hilay", "ott", "playlist", "distro", "pluto", "klowd", "wns", "local", "latest"}
	for _, p := range providers {
		if strings.Contains(c, p) {
			c = "" // Mark for reset to General
			break
		}
	}

	if c == "" || c == "undefined" || c == "general" || c == "other" || c == "channels" {
		c = "General"
	}

	// Infer from Name: If it's General, try to find a better category in the Name
	if c == "General" {
		if strings.Contains(n, "news") {
			return "News"
		}
		if strings.Contains(n, "sport") || strings.Contains(n, "cricket") || strings.Contains(n, "football") {
			return "Sports"
		}
		if strings.Contains(n, "movie") || strings.Contains(n, "cinema") || strings.Contains(n, "film") {
			return "Movies"
		}
		if strings.Contains(n, "music") || strings.Contains(n, "song") {
			return "Music"
		}
		if strings.Contains(n, "kids") || strings.Contains(n, "children") || strings.Contains(n, "cartoon") {
			return "Kids"
		}
		if strings.Contains(n, "islam") || strings.Contains(n, "quran") || strings.Contains(n, "religion") {
			return "Religious"
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
		"cartoon":       "Kids",
		"sports":        "Sports",
		"sport":         "Sports",
		"football":      "Sports",
		"cricket":       "Sports",
		"news":          "News",
		"music":         "Music",
		"religious":     "Religious",
		"religion":      "Religious",
		"islamic":       "Religious",
		"islam":         "Religious",
		"comedy":        "Comedy",
		"business":      "Business",
		"bangladeshi":   "Bangla",
		"bangla":        "Bangla",
		"kolkata":       "Bangla",
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
