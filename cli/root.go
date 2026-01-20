// Package cli provides the command-line interface for GopherQueue.
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Version information (set via ldflags)
var (
	Version   = "0.1.0"
	BuildDate = "unknown"
	GitCommit = "unknown"
)

// Global flags
var (
	cfgFile    string
	dataDir    string
	serverAddr string
	apiKey     string
	verbose    bool
)

// rootCmd is the base command.
var rootCmd = &cobra.Command{
	Use:   "gq",
	Short: "GopherQueue - Enterprise Background Job Engine",
	Long: `GopherQueue is an enterprise-grade, local-first background job engine.

It provides reliable job processing with features like:
  - Priority queues
  - Automatic retries with backoff
  - Job dependencies
  - Checkpointing for long-running jobs
  - Full observability

Use "gq [command] --help" for more information about a command.`,
	Version: fmt.Sprintf("%s (built %s, commit %s)", Version, BuildDate, GitCommit),
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// Persistent flags (available to all commands)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: $HOME/.gopherqueue.yaml)")
	rootCmd.PersistentFlags().StringVar(&dataDir, "data-dir", "./data", "data directory for persistence")
	rootCmd.PersistentFlags().StringVar(&serverAddr, "server", "http://localhost:8080", "server address for client commands")
	rootCmd.PersistentFlags().StringVar(&apiKey, "api-key", "", "API key for authentication")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")

	// Add subcommands
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(submitCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(versionCmd)
}

// versionCmd prints version information.
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("GopherQueue %s\n", Version)
		fmt.Printf("  Built:  %s\n", BuildDate)
		fmt.Printf("  Commit: %s\n", GitCommit)
	},
}
