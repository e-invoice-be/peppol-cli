package cli

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// confirm asks a yes/no question and reports whether the user answered
// "y" or "yes" (case-insensitive). The prompt and the "Aborted." notice go
// to stderr so stdout stays reserved for command output (in particular the
// single JSON document in --json mode). The answer is read from
// cmd.InOrStdin() so tests can supply it.
func confirm(cmd *cobra.Command, question string) bool {
	errOut := cmd.ErrOrStderr()
	fmt.Fprintf(errOut, "%s [y/N] ", question)
	line, _ := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	answer := strings.ToLower(strings.TrimSpace(line))
	if answer == "y" || answer == "yes" {
		return true
	}
	fmt.Fprintln(errOut, "Aborted.")
	return false
}
