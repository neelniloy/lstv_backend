package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// Channel represents a grouped channel in the output JSON.
type Channel struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Category string   `json:"category"`
	Logo     string   `json:"logo"`
	EPGID    string   `json:"epg_id,omitempty"`
	Servers  []Server `json:"servers"`
}

// Server represents a stream server with URL and latency.
type Server struct {
	URL       string  `json:"url"`
	Latency   int64   `json:"latency"`
	Quality   string  `json:"quality"`
	Stability float64 `json:"stability"`
	Type      string  `json:"type"`
}

// GenerateJSON groups entries and writes them to channels.json.
func GenerateJSON(entries []StreamEntry, outputPath string) error {
	// Grouping
	groups := make(map[string]*Channel)
	serverURLs := make(map[string]map[string]bool) // key → set of server URLs for O(1) dedup

	for _, entry := range entries {
		// Grouping key
		key := SimplifyForID(entry.Name)

		if key == "" {
			continue
		}

		if _, exists := groups[key]; !exists {
			groups[key] = &Channel{
				ID:       key,
				Name:     entry.Name,
				Category: entry.Category,
				Logo:     entry.Logo,
				EPGID:    entry.TvgID, // Use TvgID as initial EPGID if available
				Servers:  []Server{},
			}
			serverURLs[key] = make(map[string]bool)
		} else {
			// Metadata Prioritization
			if groups[key].Logo == "" && entry.Logo != "" {
				groups[key].Logo = entry.Logo
			}
			if groups[key].EPGID == "" && entry.TvgID != "" {
				groups[key].EPGID = entry.TvgID
			}
			// Prefer names that are longer or have better casing
			currentName := groups[key].Name
			newName := entry.Name
			if len(newName) > len(currentName) || (currentName == strings.ToUpper(currentName) && newName != strings.ToUpper(newName)) {
				groups[key].Name = newName
			}
		}

		// O(1) duplicate URL check
		if !serverURLs[key][entry.URL] {
			serverURLs[key][entry.URL] = true
			groups[key].Servers = append(groups[key].Servers, Server{
				URL:       entry.URL,
				Latency:   entry.Latency,
				Quality:   entry.Quality,
				Stability: entry.Stability,
				Type:      entry.Type,
			})
		}
	}

	var channels []Channel
	for _, ch := range groups {
		// Priority Sorting:
		// 1. Stability (highest first)
		// 2. Latency (lowest first)
		// 3. Quality (FHD > HD > SD)
		sort.Slice(ch.Servers, func(i, j int) bool {
			s1, s2 := ch.Servers[i], ch.Servers[j]
			if s1.Stability != s2.Stability {
				return s1.Stability > s2.Stability
			}
			if s1.Latency != s2.Latency {
				return s1.Latency < s2.Latency
			}
			qPriority := map[string]int{"4K": 4, "FHD": 3, "HD": 2, "SD": 1, "": 0}
			return qPriority[s1.Quality] > qPriority[s2.Quality]
		})

		// Limit to 10 servers
		if len(ch.Servers) > 10 {
			ch.Servers = ch.Servers[:10]
		}

		channels = append(channels, *ch)
	}

	// Sort channels alphabetically by name
	sort.Slice(channels, func(i, j int) bool {
		return channels[i].Name < channels[j].Name
	})

	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(channels); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	fmt.Printf("Successfully generated %d channels in %s.\n", len(channels), outputPath)
	return nil
}
