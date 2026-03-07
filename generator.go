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
	Name     string   `json:"name"`
	Category string   `json:"category"`
	Logo     string   `json:"logo"`
	Servers  []Server `json:"servers"`
}

// Server represents a stream server with URL and latency.
type Server struct {
	URL     string `json:"url"`
	Latency int64  `json:"latency"`
}

// GenerateJSON groups entries and writes them to channels.json.
func GenerateJSON(entries []StreamEntry, outputPath string) error {
	// Grouping
	groups := make(map[string]*Channel)

	for _, entry := range entries {
		// Grouping key: normalized name primarily to merge all servers for that channel
		key := entry.Name

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
			// Prefer names that aren't empty and logos that aren't empty
			if groups[key].Logo == "" && entry.Logo != "" {
				groups[key].Logo = entry.Logo
			}
			if len(entry.Name) > len(groups[key].Name) && !strings.Contains(entry.Name, "()") {
				groups[key].Name = entry.Name
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
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(channels); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	fmt.Printf("Successfully generated %d channels in %s.\n", len(channels), outputPath)
	return nil
}
