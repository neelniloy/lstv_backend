package main

import (
	"flag"
	"fmt"
	"os"
	"time"
)

func runPipeline() {
	fmt.Printf("\n--- Starting Pipeline: %s ---\n", time.Now().Format(time.RFC3339))

	// 1. Fetch Playlists
	playlists, err := FetchPlaylists("sources.txt")
	if err != nil {
		fmt.Printf("Error fetching playlists: %v\n", err)
		return
	}

	// 2. Parse and Normalize
	var allEntries []StreamEntry
	for _, content := range playlists {
		entries := ParseM3U(content)
		for i := range entries {
			entries[i].Name = NormalizeName(entries[i].Name)
			entries[i].Category = NormalizeCategory(entries[i].Category, entries[i].Name)
		}
		allEntries = append(allEntries, entries...)
	}
	fmt.Printf("Total entries parsed: %d\n", len(allEntries))

	// 3. Health Check
	fmt.Println("Running health checks on streams...")
	healthyEntries := RunHealthChecks(allEntries, 100)

	// 4. Generate JSON
	err = GenerateJSON(healthyEntries, "channels.json")
	if err != nil {
		fmt.Printf("Error generating output: %v\n", err)
		return
	}

	fmt.Printf("--- Pipeline Completed: %v ---\n", time.Now().Format(time.RFC3339))
}

func main() {
	cronMode := flag.Bool("cron", false, "Run pipeline once and exit (for GitHub Actions)")
	flag.Parse()

	if *cronMode {
		fmt.Println("Running in CRON mode (one-shot)")
		runPipeline()
		os.Exit(0)
	}

	fmt.Println("IPTV Backend Processor Started")

	// Run initially
	runPipeline()

	// Schedule every 30 minutes
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()

	fmt.Println("Scheduler started: Pipeline will run every 30 minutes.")

	for t := range ticker.C {
		fmt.Printf("Scheduled run at %v\n", t)
		runPipeline()
	}
}
