package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

// Channel represents a grouped channel in the output JSON.
type Channel struct {
	Name     string   `json:"name"`
	Category string   `json:"category"`
	Logo     string   `json:"logo"`
	Servers  []Server `json:"servers"`
}

// Server represents a stream server with URL and latency.
type Server struct {
	URL     string `json:"url"`
	Latency int64  `json:"latency"`
	Quality string `json:"quality"`
}

var reKey = regexp.MustCompile(`[^a-z0-9]`)

// GenerateJSON groups entries and writes them to channels.json.
func GenerateJSON(entries []StreamEntry, outputPath string) error {
	// Grouping
	groups := make(map[string]*Channel)

	for _, entry := range entries {
		// Grouping key: lowercase and no spaces/symbols to merge variations
		key := strings.ToLower(entry.Name)
		key = reKey.ReplaceAllString(key, "")

		if key == "" {
			continue
		}

		if _, exists := groups[key]; !exists {
			groups[key] = &Channel{
				Name:     entry.Name,
				Category: entry.Category,
				Logo:     entry.Logo,
				Servers:  []Server{},
			}
		} else {
			// Metadata Prioritization: Update name or logo if entry has a better one
			if groups[key].Logo == "" && entry.Logo != "" {
				groups[key].Logo = entry.Logo
			}
			// Prefer names that are longer or have better casing (not all caps)
			currentName := groups[key].Name
			newName := entry.Name
			if len(newName) > len(currentName) || (currentName == strings.ToUpper(currentName) && newName != strings.ToUpper(newName)) {
				groups[key].Name = newName
			}
		}

		// Check for duplicate URLs within the same channel
		isDuplicate := false
		for _, s := range groups[key].Servers {
			if s.URL == entry.URL {
				isDuplicate = true
				break
			}
		}

		if !isDuplicate {
			groups[key].Servers = append(groups[key].Servers, Server{
				URL:     entry.URL,
				Latency: entry.Latency,
				Quality: entry.Quality,
			})
		}
	}

	var channels []Channel
	for _, ch := range groups {
		// Sort servers by latency
		sort.Slice(ch.Servers, func(i, j int) bool {
			return ch.Servers[i].Latency < ch.Servers[j].Latency
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
