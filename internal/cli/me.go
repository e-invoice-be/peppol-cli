package cli

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/e-invoicebe/peppol-cli/internal/client"
	"github.com/e-invoicebe/peppol-cli/internal/config"
	"github.com/spf13/cobra"
)

func newMeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "me",
		Short: "Display current tenant account information",
		RunE:  runMe,
	}
}

func runMe(cmd *cobra.Command, args []string) error {
	apiKey, err := resolveKey()
	if err != nil {
		return err
	}

	c := client.NewClient(apiKey)
	tenant, err := c.GetMe()
	if err != nil {
		if errors.Is(err, client.ErrUnauthorized) {
			return &ExitError{Err: fmt.Errorf("authentication failed (invalid API key)"), Code: 2}
		}
		return &ExitError{Err: err, Code: 1}
	}

	if flags.JSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(tenant)
	}

	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "Name:           %s\n", tenant.Name)
	if tenant.CompanyName != nil {
		fmt.Fprintf(w, "Company:        %s\n", *tenant.CompanyName)
	}
	if tenant.CompanyNumber != nil {
		fmt.Fprintf(w, "Company Number: %s\n", *tenant.CompanyNumber)
	}
	if tenant.CompanyTaxID != nil {
		fmt.Fprintf(w, "Tax ID:         %s\n", *tenant.CompanyTaxID)
	}
	fmt.Fprintf(w, "Plan:           %s\n", tenant.Plan)
	if len(tenant.PeppolIDs) > 0 {
		for _, id := range tenant.PeppolIDs {
			fmt.Fprintf(w, "Peppol ID:      %s\n", id)
		}
	}
	return nil
}

// resolveKey gets the API key or returns an auth exit error.
func resolveKey() (string, error) {
	kr := config.NewFileKeyring(mustConfigDir())
	key, err := config.ResolveAPIKey(kr)
	if err != nil {
		return "", &ExitError{Err: fmt.Errorf("reading credentials: %w", err), Code: 1}
	}
	if key == "" {
		return "", &ExitError{
			Err:  fmt.Errorf("not authenticated — run 'peppol auth' or set PEPPOL_API_KEY"),
			Code: 2,
		}
	}
	return key, nil
}

func mustConfigDir() string {
	dir, err := config.ConfigDir()
	if err != nil {
		return ""
	}
	return dir
}
