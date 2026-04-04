package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// StreamStability stores the health history of a stream URL.
type StreamStability struct {
	URL         string `json:"url"`
	Success     int    `json:"success"`
	Total       int    `json:"total"`
	LastChecked int64  `json:"last_checked"` // unix timestamp of last health check
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

const stabilityWindow = 20 // rolling window: only track last N checks per URL

// pruneStability removes dead entries and caps Total to the rolling window.
// Must be called with stabilityMu write-locked.
func pruneStability() {
	for url, s := range stabilityMap {
		// Evict URLs that have never worked after enough tries
		if s.Total >= 10 && s.Success == 0 {
			delete(stabilityMap, url)
			continue
		}
		// Cap to rolling window so recent failures lower the score quickly
		if s.Total > stabilityWindow {
			excess := s.Total - stabilityWindow
			s.Total = stabilityWindow
			if s.Success > excess {
				s.Success -= excess
			} else {
				s.Success = 0
			}
		}
	}
}

// SaveStability saves historical stability data to stability.json.
func SaveStability() error {
	stabilityMu.Lock()
	pruneStability()
	stabilityMu.Unlock()

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
	s.LastChecked = time.Now().Unix()

	if s.Total == 0 {
		return 0
	}
	return float64(s.Success) / float64(s.Total)
}

// ShouldSkipCheck returns true if the stream was recently checked and has high
// stability — we can safely assume it's still live and skip the network check.
// Only effective in daemon mode where stability data persists between runs.
func ShouldSkipCheck(url string) bool {
	stabilityMu.RLock()
	defer stabilityMu.RUnlock()

	s, exists := stabilityMap[url]
	if !exists || s.LastChecked == 0 || s.Total == 0 {
		return false
	}

	age := time.Now().Unix() - s.LastChecked
	stability := float64(s.Success) / float64(s.Total)

	// Skip if checked within 20 minutes AND very stable (>= 0.8)
	return age < 20*60 && stability >= 0.8
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
