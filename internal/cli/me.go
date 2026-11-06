package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/e-invoice-be/peppol-cli/internal/client"
	"github.com/e-invoice-be/peppol-cli/internal/config"
	"github.com/e-invoice-be/peppol-cli/internal/output"
)

func newMeCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "me",
		Short:   "Display current tenant account information",
		Example: "  peppol me\n  peppol me --json",
		RunE:    runMe,
	}
}

func runMe(cmd *cobra.Command, args []string) error {
	apiKey, err := resolveKey()
	if err != nil {
		return err
	}

	c := client.NewClient(apiKey, clientOpts()...).WithContext(cmd.Context())
	tenant, err := c.GetMe()
	if err != nil {
		if errors.Is(err, client.ErrUnauthorized) {
			return &ExitError{Err: errors.New("authentication failed (invalid API key)"), Code: 2}
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

// resolveKey gets the API key for the active (or flag-overridden) workspace.
func resolveKey() (string, error) {
	dir := mustConfigDir()

	// Determine which workspace to use.
	workspace := flags.Workspace
	if workspace == "" {
		cfg, err := loadConfig(dir)
		if err != nil {
			return "", err
		}
		workspace = cfg.ActiveWorkspace
	} else if err := config.ValidateWorkspaceName(workspace); err != nil {
		// Legacy names outside the allowed set are still accepted when they
		// exist in the config.
		cfg, loadErr := config.LoadFrom(dir)
		if loadErr != nil {
			return "", &ExitError{Err: err, Code: 1}
		}
		if err := checkWorkspaceName(workspace, cfg); err != nil {
			return "", err
		}
	}

	key, _, err := resolveKeySource(dir, workspace)
	if err != nil {
		return "", err
	}
	if key == "" {
		return "", &ExitError{
			Err:  errors.New("not authenticated — run 'peppol auth' or set PEPPOL_API_KEY"),
			Code: 2,
		}
	}
	return key, nil
}

// resolveKeySource returns the API key for workspace and where it came from.
// Both resolveKey and auth status go through it, so status always reports
// the key that commands actually use. An empty key with KeySourceNone means
// not authenticated; read errors are ExitErrors with code 1.
func resolveKeySource(dir, workspace string) (string, config.KeySource, error) {
	var kr config.KeyringBackend
	if workspace != "" {
		kr = config.NewFileKeyringForWorkspace(dir, workspace)
	} else {
		kr = config.NewFileKeyring(dir)
	}
	key, source, err := config.ResolveAPIKeySource(kr)
	if err != nil {
		return "", config.KeySourceNone, &ExitError{Err: fmt.Errorf("reading credentials: %w", err), Code: 1}
	}
	return key, source, nil
}

func mustConfigDir() string {
	dir, err := config.ConfigDir()
	if err != nil {
		return ""
	}
	return dir
}
