// Package cli provides the submit command for GopherQueue.
package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"
)

var (
	jobType        string
	jobPayload     string
	jobPayloadFile string
	jobPriority    int
	jobDelay       string
	jobTimeout     string
	jobMaxAttempts int
	jobIdempotency string
	jobCorrelation string
	jobTags        []string
)

var submitCmd = &cobra.Command{
	Use:   "submit",
	Short: "Submit a new job",
	Long: `Submit a new job to the GopherQueue server.

Examples:
  # Submit a simple echo job
  gq submit --type echo --payload '{"message": "hello"}'
  
  # Submit with priority and delay
  gq submit --type report --payload '{"id": 123}' --priority 1 --delay 5m
  
  # Submit with idempotency key
  gq submit --type email --payload @email.json --idempotency-key "email-12345"`,
	RunE: runSubmit,
}

func init() {
	submitCmd.Flags().StringVarP(&jobType, "type", "t", "", "job type (required)")
	submitCmd.Flags().StringVarP(&jobPayload, "payload", "p", "{}", "job payload (JSON)")
	submitCmd.Flags().StringVarP(&jobPayloadFile, "payload-file", "f", "", "read payload from file")
	submitCmd.Flags().IntVar(&jobPriority, "priority", 2, "job priority (0=critical, 4=bulk)")
	submitCmd.Flags().StringVar(&jobDelay, "delay", "", "delay before processing (e.g., 5m, 1h)")
	submitCmd.Flags().StringVar(&jobTimeout, "timeout", "", "job timeout (e.g., 30m, 1h)")
	submitCmd.Flags().IntVar(&jobMaxAttempts, "max-attempts", 3, "maximum retry attempts")
	submitCmd.Flags().StringVar(&jobIdempotency, "idempotency-key", "", "idempotency key for deduplication")
	submitCmd.Flags().StringVar(&jobCorrelation, "correlation-id", "", "correlation ID for tracing")
	submitCmd.Flags().StringSliceVar(&jobTags, "tag", nil, "tags in key=value format")

	_ = submitCmd.MarkFlagRequired("type")
}

func runSubmit(cmd *cobra.Command, args []string) error {
	// Read payload
	payload := []byte(jobPayload)
	if jobPayloadFile != "" {
		if jobPayloadFile == "-" {
			var err error
			payload, err = io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("failed to read from stdin: %w", err)
			}
		} else {
			var err error
			payload, err = os.ReadFile(jobPayloadFile)
			if err != nil {
				return fmt.Errorf("failed to read payload file: %w", err)
			}
		}
	}

	// Parse tags
	tags := make(map[string]string)
	for _, tag := range jobTags {
		var key, value string
		if n, _ := fmt.Sscanf(tag, "%s=%s", &key, &value); n == 2 {
			tags[key] = value
		}
	}

	// Build request
	reqBody := map[string]interface{}{
		"type":         jobType,
		"payload":      json.RawMessage(payload),
		"priority":     jobPriority,
		"max_attempts": jobMaxAttempts,
	}

	if jobDelay != "" {
		reqBody["delay"] = jobDelay
	}
	if jobTimeout != "" {
		reqBody["timeout"] = jobTimeout
	}
	if jobIdempotency != "" {
		reqBody["idempotency_key"] = jobIdempotency
	}
	if jobCorrelation != "" {
		reqBody["correlation_id"] = jobCorrelation
	}
	if len(tags) > 0 {
		reqBody["tags"] = tags
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Make request
	url := fmt.Sprintf("%s/api/v1/jobs", serverAddr)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to submit job: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned %d: %s", resp.StatusCode, string(body))
	}

	// Pretty print response
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Println(string(body))
		return nil
	}

	if verbose {
		prettyJSON, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(prettyJSON))
	} else {
		if id, ok := result["id"].(string); ok {
			fmt.Printf("Job submitted: %s\n", id)
		}
		if state, ok := result["state"].(string); ok {
			fmt.Printf("  State: %s\n", state)
		}
	}

	return nil
}
