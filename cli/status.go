// Package cli provides the status command for GopherQueue.
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

var (
	statusJobID  string
	statusState  string
	statusType   string
	statusFormat string
)

var statusCmd = &cobra.Command{
	Use:   "status [job-id]",
	Short: "Get job status or list jobs",
	Long: `Query job status or list jobs with filters.

Examples:
  # Get status of a specific job
  gq status abc123-def456
  
  # List all pending jobs
  gq status --state pending
  
  # List jobs by type
  gq status --type email
  
  # Get server stats
  gq status --stats`,
	RunE: runStatus,
}

var showStats bool

func init() {
	statusCmd.Flags().StringVar(&statusState, "state", "", "filter by state (pending, running, completed, failed)")
	statusCmd.Flags().StringVar(&statusType, "type", "", "filter by job type")
	statusCmd.Flags().StringVar(&statusFormat, "format", "table", "output format (table, json)")
	statusCmd.Flags().BoolVar(&showStats, "stats", false, "show server statistics")
}

func runStatus(cmd *cobra.Command, args []string) error {
	if showStats {
		return showServerStats()
	}

	if len(args) > 0 {
		return getJobStatus(args[0])
	}

	return listJobs()
}

func getJobStatus(jobID string) error {
	url := fmt.Sprintf("%s/api/v1/jobs/%s", serverAddr, jobID)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to get job status: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("job not found: %s", jobID)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned %d: %s", resp.StatusCode, string(body))
	}

	var job map[string]interface{}
	if err := json.Unmarshal(body, &job); err != nil {
		return err
	}

	if statusFormat == "json" {
		prettyJSON, _ := json.MarshalIndent(job, "", "  ")
		fmt.Println(string(prettyJSON))
		return nil
	}

	// Table format
	fmt.Printf("Job: %s\n", job["id"])
	fmt.Printf("  Type:        %s\n", job["type"])
	fmt.Printf("  State:       %s\n", job["state"])
	fmt.Printf("  Priority:    %.0f\n", job["priority"])
	fmt.Printf("  Attempts:    %.0f/%.0f\n", job["attempt"], job["max_attempts"])
	fmt.Printf("  Created:     %s\n", job["created_at"])
	fmt.Printf("  Updated:     %s\n", job["updated_at"])

	if progress, ok := job["progress"].(float64); ok && progress > 0 {
		fmt.Printf("  Progress:    %.1f%%\n", progress)
	}
	if msg, ok := job["progress_message"].(string); ok && msg != "" {
		fmt.Printf("  Message:     %s\n", msg)
	}
	if err, ok := job["last_error"].(string); ok && err != "" {
		fmt.Printf("  Last Error:  %s\n", err)
	}

	return nil
}

func listJobs() error {
	url := fmt.Sprintf("%s/api/v1/jobs", serverAddr)

	// Build query params
	params := []string{}
	if statusState != "" {
		params = append(params, fmt.Sprintf("state=%s", statusState))
	}
	if statusType != "" {
		params = append(params, fmt.Sprintf("type=%s", statusType))
	}
	if len(params) > 0 {
		url = url + "?" + strings.Join(params, "&")
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to list jobs: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Jobs  []map[string]interface{} `json:"jobs"`
		Count int                      `json:"count"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return err
	}

	if statusFormat == "json" {
		prettyJSON, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(prettyJSON))
		return nil
	}

	if len(result.Jobs) == 0 {
		fmt.Println("No jobs found")
		return nil
	}

	// Table format
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tTYPE\tSTATE\tPRIORITY\tATTEMPTS\tCREATED")
	fmt.Fprintln(w, "--\t----\t-----\t--------\t--------\t-------")

	for _, job := range result.Jobs {
		id := truncate(fmt.Sprintf("%v", job["id"]), 36)
		jobType := fmt.Sprintf("%v", job["type"])
		state := fmt.Sprintf("%v", job["state"])
		priority := fmt.Sprintf("%.0f", job["priority"])
		attempts := fmt.Sprintf("%.0f/%.0f", job["attempt"], job["max_attempts"])
		created := formatTime(job["created_at"])

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", id, jobType, state, priority, attempts, created)
	}
	w.Flush()

	fmt.Printf("\nTotal: %d jobs\n", result.Count)
	return nil
}

func showServerStats() error {
	url := fmt.Sprintf("%s/api/v1/stats", serverAddr)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to get stats: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned %d: %s", resp.StatusCode, string(body))
	}

	var stats map[string]interface{}
	if err := json.Unmarshal(body, &stats); err != nil {
		return err
	}

	if statusFormat == "json" {
		prettyJSON, _ := json.MarshalIndent(stats, "", "  ")
		fmt.Println(string(prettyJSON))
		return nil
	}

	fmt.Println("GopherQueue Statistics")
	fmt.Println("======================")

	if queue, ok := stats["queue"].(map[string]interface{}); ok {
		fmt.Println("\nQueue:")
		fmt.Printf("  Pending:     %.0f\n", queue["pending"])
		fmt.Printf("  Scheduled:   %.0f\n", queue["scheduled"])
		fmt.Printf("  Running:     %.0f\n", queue["running"])
		fmt.Printf("  Delayed:     %.0f\n", queue["delayed"])
		fmt.Printf("  Completed:   %.0f\n", queue["completed"])
		fmt.Printf("  Failed:      %.0f\n", queue["failed"])
		fmt.Printf("  Dead Letter: %.0f\n", queue["dead_letter"])
		fmt.Printf("  Total:       %.0f\n", queue["total_jobs"])
	}

	if workers, ok := stats["workers"].(map[string]interface{}); ok {
		fmt.Println("\nWorkers:")
		fmt.Printf("  Total:       %.0f\n", workers["total_workers"])
		fmt.Printf("  Active:      %.0f\n", workers["active_workers"])
		fmt.Printf("  Idle:        %.0f\n", workers["idle_workers"])
		fmt.Printf("  Processed:   %.0f\n", workers["processed_jobs"])
		fmt.Printf("  Succeeded:   %.0f\n", workers["succeeded_jobs"])
		fmt.Printf("  Failed:      %.0f\n", workers["failed_jobs"])
	}

	return nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func formatTime(t interface{}) string {
	if t == nil {
		return "-"
	}
	str, ok := t.(string)
	if !ok {
		return "-"
	}
	parsed, err := time.Parse(time.RFC3339Nano, str)
	if err != nil {
		return str
	}
	return parsed.Format("2006-01-02 15:04:05")
}
