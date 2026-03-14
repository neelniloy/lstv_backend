package main

import (
	"regexp"
	"strings"
)

var (
	reLeadingDigits   = regexp.MustCompile(`^\d{3,}\s+`)
	reAmpersand       = regexp.MustCompile(`\s*&\s*`)
	reBrackets        = regexp.MustCompile(`(?i)\[[^\]]*\]`)
	reParentheses     = regexp.MustCompile(`(?i)\([^\)]*\)`)
	reSymbols         = regexp.MustCompile(`[^\w\s\-\&]`)
	reMultipleSpaces  = regexp.MustCompile(`\s+`)
	tagRegexes        []*regexp.Regexp
	wordReplacements  = map[string]string{
		"Minutes": "Minute",
		"Crafts":  "Craft",
	}
	wordRegexes = make(map[string]*regexp.Regexp)
)

func init() {
	tags := []string{
		`(?i)\bHD\b`,
		`(?i)\bFHD\b`,
		`(?i)\bSD\b`,
		`(?i)\bUHD\b`,
		`(?i)\b4K\b`,
		`(?i)\b\d+p\b`,
		`(?i)\b\d+i\b`,
		`(?i)\bLIVE\b`,
		`(?i)\bBACKUP\b`,
		`(?i)\bOFFICIAL\b`,
		`(?i)\bSTREAM\b`,
		`(?i)\bONLINE\b`,
		`(?i)\bENG\b`,
		`(?i)\bITA\b`,
		`(?i)\bESP\b`,
		`(?i)\bFRA\b`,
		`(?i)\bHIN\b`,
		`(?i)\bKOR\b`,
		`(?i)\bJPN\b`,
	}
	for _, tag := range tags {
		tagRegexes = append(tagRegexes, regexp.MustCompile(tag))
	}

	for old := range wordReplacements {
		wordRegexes[old] = regexp.MustCompile(`(?i)\b` + old + `\b`)
	}
}

// DetectQuality identifies the stream quality (4K, FHD, HD, SD) from the channel name or URL.
func DetectQuality(name string, url string) string {
	combined := strings.ToUpper(name + " " + url)
	if strings.Contains(combined, "4K") || strings.Contains(combined, "UHD") || strings.Contains(combined, "2160P") {
		return "4K"
	}
	if strings.Contains(combined, "FHD") || strings.Contains(combined, "1080P") || strings.Contains(combined, "1080I") {
		return "FHD"
	}
	if strings.Contains(combined, "HD") || strings.Contains(combined, "720P") || strings.Contains(combined, "720I") {
		return "HD"
	}
	if strings.Contains(combined, "SD") || strings.Contains(combined, "576I") || strings.Contains(combined, "480P") || strings.Contains(combined, "VGA") {
		return "SD"
	}
	return ""
}

// NormalizeName removes quality indicators and other tags from the channel name.
func NormalizeName(name string) string {
	normalized := name

	// 1. Unify separators (hyphens/underscores/slashes to spaces)
	normalized = strings.ReplaceAll(normalized, "-", " ")
	normalized = strings.ReplaceAll(normalized, "_", " ")
	normalized = strings.ReplaceAll(normalized, "/", " ")

	// 2. Strip leading digits followed by space (only if 3+ digits, e.g., "101 NBC" -> "NBC")
	normalized = reLeadingDigits.ReplaceAllString(normalized, "")

	// 3. Tags to remove
	normalized = reBrackets.ReplaceAllString(normalized, "")
	normalized = reParentheses.ReplaceAllString(normalized, "")

	for _, re := range tagRegexes {
		normalized = re.ReplaceAllString(normalized, "")
	}

	// 5. Unify common word variations
	for old, new := range wordReplacements {
		normalized = wordRegexes[old].ReplaceAllString(normalized, new)
	}

	// 6. Clean up extra spaces/symbols
	normalized = reSymbols.ReplaceAllString(normalized, "") // Keep only alphanumeric, spaces, hyphens and ampersands
	normalized = reMultipleSpaces.ReplaceAllString(normalized, " ")
	normalized = strings.TrimSpace(normalized)

	// 7. Master Name Mapping (Only for exact/near-exact fixes, not greedy)
	nameMappings := map[string]string{
		"BBC NEWS":     "BBC News",
		"BBC WORLD":    "BBC News",
		"NAT GEO":      "National Geographic",
		"NATGEO":       "National Geographic",
		"ZEE TV":       "Zee TV",
		"SONY TV":      "Sony TV",
		"STAR JALSHA":  "Star Jalsha",
		"STAR PLUS":    "Star Plus",
		"COLORS TV":    "Colors TV",
		"AND TV":       "& TV",
		"& TV":         "& TV",
		"&TV":          "& TV",
		"AND PICTURES": "& Pictures",
		"AND PICTURS":  "& Pictures",
		"& PICTURES":   "& Pictures",
		"&PICTURES":    "& Pictures",
		"AND FLIX":     "& Flix",
		"& FLIX":       "& Flix",
		"AND PRIVE":    "& Prive",
		"& PRIVE":      "& Prive",
		"AND XPLORE":   "& Xplore",
		"AND XPLOR":    "& Xplore",
		"& XPLORE":     "& Xplore",
		"& XPLOR":      "& Xplore",
	}

	upperNormalized := strings.ToUpper(normalized)
	// Only map if the name is a very close match to prevent "Star Jalsha Movies" -> "Star Jalsha"
	if standard, exists := nameMappings[upperNormalized]; exists {
		return standard // Return immediately to avoid title casing logic
	}

	// 8. Robust Casing
	if normalized == "" {
		return ""
	}

	// Known acronyms that should always be uppercase
	acronyms := map[string]bool{
		"B4U":    true, "MTV": true, "HBO": true, "CNN": true, "BBC": true,
		"NDTV":   true, "CNBC": true, "ESPN": true, "TNT": true, "AMC": true,
		"AXN":    true, "SET": true, "TVP": true, "TLC": true, "SD": true,
		"UHD":    true, "FHD": true, "NGC": true, "STAR": true, "ZEE": true,
		"ABP":    true, "PTC": true, "WB": true, "FX": true, "SYFY": true,
		"VH1":    true, "CW": true, "USA": true, "WBW": true, "SMC": true,
		"NHK":    true, "CGTN": true, "CNA": true, "DW": true, "RT": true,
		"ALJ":    true, "BTV": true, "DBC": true, "ATN": true, "NTV": true,
		"RTV":    true, "SA": true, "9X": true, "SONY": true, "TEN": true,
		"&TV":    true, "&XPLOR": true, "& TV": true,
	}

	words := strings.Fields(normalized)
	for i, word := range words {
		upperWord := strings.ToUpper(word)
		if acronyms[upperWord] {
			words[i] = upperWord
		} else {
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

// NormalizeCategory standardizes category names into a fixed list.
func NormalizeCategory(category string, channelName string) string {
	parts := strings.Split(category, ";")
	if len(parts) > 1 {
		category = parts[0]
	}

	c := strings.ToLower(strings.TrimSpace(category))
	n := strings.ToLower(channelName)

	// Required List: Sports, Movies, News, Kids, Entertainment, Music, Documentary, Religious, International, General

	// 1. Mandatory Name-Based Overrides
	if strings.Contains(n, "sport") || strings.Contains(n, "cricket") || strings.Contains(n, "football") || strings.Contains(n, "ten ") || strings.Contains(n, "europort") || strings.Contains(n, "fancode") {
		return "Sports"
	}
	if strings.Contains(n, "movie") || strings.Contains(n, "cinema") || strings.Contains(n, "film") || strings.Contains(n, "action") || strings.Contains(n, "prive") || strings.Contains(n, "flix") || strings.Contains(n, "xplore") || strings.Contains(n, "hbo") {
		return "Movies"
	}
	if strings.Contains(n, "news") || strings.Contains(n, "times") || strings.Contains(n, "reuters") || strings.Contains(n, "cnn") || strings.Contains(n, "bbc") {
		return "News"
	}
	if strings.Contains(n, "kids") || strings.Contains(n, "cartoon") || strings.Contains(n, "nick") || strings.Contains(n, "disney") || strings.Contains(n, "pogo") || strings.Contains(n, "sony yay") {
		return "Kids"
	}
	if strings.Contains(n, "music") || strings.Contains(n, "song") || strings.Contains(n, "vh1") || strings.Contains(n, "mix") || strings.Contains(n, "zoom") || strings.Contains(n, "9x") {
		return "Music"
	}
	if strings.Contains(n, "docu") || strings.Contains(n, "geo") || strings.Contains(n, "history") || strings.Contains(n, "discovery") || strings.Contains(n, "animal") || strings.Contains(n, "planet") {
		return "Documentary"
	}
	if strings.Contains(n, "islam") || strings.Contains(n, "quran") || strings.Contains(n, "religion") || strings.Contains(n, "peace tv") || strings.Contains(n, "god") || strings.Contains(n, "church") {
		return "Religious"
	}
	if strings.Contains(n, "uk") || strings.Contains(n, "usa") || strings.Contains(n, "france") || strings.Contains(n, "germany") || strings.Contains(n, "global") || strings.Contains(n, "international") {
		return "International"
	}

	// 2. Map existing category strings
	mappings := map[string]string{
		"sport":         "Sports",
		"movie":         "Movies",
		"cinema":        "Movies",
		"film":          "Movies",
		"news":          "News",
		"kids":          "Kids",
		"cartoon":       "Kids",
		"disney":        "Kids",
		"music":         "Music",
		"video":         "Music",
		"documentary":   "Documentary",
		"geo":           "Documentary",
		"nature":        "Documentary",
		"religion":      "Religious",
		"religious":     "Religious",
		"islam":         "Religious",
		"international": "International",
		"world":         "International",
		"foreign":       "International",
		"entertainment": "Entertainment",
		"general":       "General",
	}

	for key, val := range mappings {
		if strings.Contains(c, key) {
			return val
		}
	}

	// 3. Fallback for common local categories to Entertainment
	if strings.Contains(c, "bangla") || strings.Contains(c, "hindi") || strings.Contains(c, "indian") || strings.Contains(c, "drama") || strings.Contains(c, "comedy") || strings.Contains(c, "series") {
		return "Entertainment"
	}

	return "General"
}

// SimplifyForID creates a unique identifier from a name (lowercase, no spaces/special chars)
func SimplifyForID(name string) string {
	s := strings.ToLower(name)
	s = strings.ReplaceAll(s, "&", "and")
	s = reKey.ReplaceAllString(s, "")
	return s
}
