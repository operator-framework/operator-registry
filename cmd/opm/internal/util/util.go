package util

import (
	"fmt"

	"github.com/spf13/cobra"
)

// GetTLSOptions validates and returns TLS options set by opm flags
func GetTLSOptions(cmd *cobra.Command) (bool, bool, error) {
	skipTLS, err := cmd.Flags().GetBool("skip-tls")
	if err != nil {
		return false, false, err
	}
	skipTLSVerify, err := cmd.Flags().GetBool("skip-tls-verify")
	if err != nil {
		return false, false, err
	}
	useHTTP, err := cmd.Flags().GetBool("use-http")
	if err != nil {
		return false, false, err
	}

	switch {
	case cmd.Flags().Changed("skip-tls") && cmd.Flags().Changed("use-http"):
		return false, false, fmt.Errorf("invalid flag combination: cannot use --use-http with --skip-tls")
	case cmd.Flags().Changed("skip-tls") && cmd.Flags().Changed("skip-tls-verify"):
		return false, false, fmt.Errorf("invalid flag combination: cannot use --skip-tls-verify with --skip-tls")
	case skipTLSVerify && useHTTP:
		return false, false, fmt.Errorf("invalid flag combination: --use-http and --skip-tls-verify cannot both be true")
	default:
		// return use HTTP true if just skipTLS
		// is set for functional parity with existing
		if skipTLS {
			return false, true, nil
		}
		return skipTLSVerify, useHTTP, nil
	}
}
