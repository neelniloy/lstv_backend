package main

import (
	"bufio"
	"regexp"
	"strings"
)

// StreamEntry represents a single stream from an M3U file.
type StreamEntry struct {
	Name     string `json:"name"`
	TvgID    string `json:"tvg_id"`
	Category string `json:"category"`
	Logo     string `json:"logo"`
	URL      string `json:"url"`
	Latency  int64  `json:"latency"`
	Quality  string `json:"quality"`
}

var (
	reTvgID = regexp.MustCompile(`tvg-id="([^"]*)"`)
	reLogo  = regexp.MustCompile(`tvg-logo="([^"]*)"`)
	reGroup = regexp.MustCompile(`group-title="([^"]*)"`)
)

// ParseM3U parses M3U playlist content and returns a list of stream entries.
func ParseM3U(content string) []StreamEntry {
	var entries []StreamEntry
	scanner := bufio.NewScanner(strings.NewReader(content))

	var currentEntry StreamEntry
	hasEntry := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#EXTINF:") {
			// Extract tags
			tvgIDMatch := reTvgID.FindStringSubmatch(line)
			if len(tvgIDMatch) > 1 {
				currentEntry.TvgID = tvgIDMatch[1]
			}

			logoMatch := reLogo.FindStringSubmatch(line)
			if len(logoMatch) > 1 {
				currentEntry.Logo = logoMatch[1]
			}

			groupMatch := reGroup.FindStringSubmatch(line)
			if len(groupMatch) > 1 {
				currentEntry.Category = groupMatch[1]
			}

			// Extract name (after the last comma)
			lastComma := strings.LastIndex(line, ",")
			if lastComma != -1 {
				currentEntry.Name = strings.TrimSpace(line[lastComma+1:])
			}
			hasEntry = true
		} else if hasEntry && line != "" && !strings.HasPrefix(line, "#") {
			currentEntry.URL = line
			entries = append(entries, currentEntry)
			// Reset for next entry
			currentEntry = StreamEntry{}
			hasEntry = false
		}
	}

	return entries
}
