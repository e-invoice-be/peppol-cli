package cli

import (
	"errors"
	"fmt"

	"github.com/e-invoicebe/peppol-cli/internal/client"
	"github.com/e-invoicebe/peppol-cli/internal/config"
	"github.com/e-invoicebe/peppol-cli/internal/output"
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

	r := output.FromContext(cmd.Context())

	if r.IsJSON() {
		return r.JSON(tenant)
	}

	pairs := []output.KVPair{
		{Key: "Name", Value: tenant.Name},
	}
	if tenant.CompanyName != nil {
		pairs = append(pairs, output.KVPair{Key: "Company", Value: *tenant.CompanyName})
	}
	if tenant.CompanyNumber != nil {
		pairs = append(pairs, output.KVPair{Key: "Company Number", Value: *tenant.CompanyNumber})
	}
	if tenant.CompanyTaxID != nil {
		pairs = append(pairs, output.KVPair{Key: "Tax ID", Value: *tenant.CompanyTaxID})
	}
	pairs = append(pairs, output.KVPair{Key: "Plan", Value: tenant.Plan})
	for _, id := range tenant.PeppolIDs {
		pairs = append(pairs, output.KVPair{Key: "Peppol ID", Value: id})
	}
	return r.KeyValue(pairs)
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
