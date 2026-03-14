package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// StreamStability stores the health history of a stream URL.
type StreamStability struct {
	URL      string `json:"url"`
	Success  int    `json:"success"`
	Total    int    `json:"total"`
}

var (
	stabilityMap = make(map[string]*StreamStability)
	stabilityMu  sync.RWMutex
	stabilityFile = "stability.json"
)

// LoadStability loads historical stability data from stability.json.
func LoadStability() error {
	stabilityMu.Lock()
	defer stabilityMu.Unlock()

	file, err := os.Open(stabilityFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // File doesn't exist yet, it's fine
		}
		return err
	}
	defer file.Close()

	var stabilities []StreamStability
	if err := json.NewDecoder(file).Decode(&stabilities); err != nil {
		return err
	}

	for _, s := range stabilities {
		sCopy := s
		stabilityMap[s.URL] = &sCopy
	}

	fmt.Printf("Loaded stability data for %d streams.\n", len(stabilityMap))
	return nil
}

// SaveStability saves historical stability data to stability.json.
func SaveStability() error {
	stabilityMu.RLock()
	defer stabilityMu.RUnlock()

	var stabilities []StreamStability
	for _, s := range stabilityMap {
		stabilities = append(stabilities, *s)
	}

	file, err := os.Create(stabilityFile)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(stabilities)
}

// UpdateStability updates the success/total counts for a URL and returns the new stability score.
func UpdateStability(url string, success bool) float64 {
	stabilityMu.Lock()
	defer stabilityMu.Unlock()

	s, exists := stabilityMap[url]
	if !exists {
		s = &StreamStability{URL: url}
		stabilityMap[url] = s
	}

	s.Total++
	if success {
		s.Success++
	}

	if s.Total == 0 {
		return 0
	}
	return float64(s.Success) / float64(s.Total)
}

// GetStability returns the current stability score for a URL.
func GetStability(url string) float64 {
	stabilityMu.RLock()
	defer stabilityMu.RUnlock()

	s, exists := stabilityMap[url]
	if !exists {
		return 1.0 // Assume healthy if new, will be updated soon
	}

	if s.Total == 0 {
		return 0
	}
	return float64(s.Success) / float64(s.Total)
}
