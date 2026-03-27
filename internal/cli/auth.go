package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/e-invoicebe/peppol-cli/internal/client"
	"github.com/e-invoicebe/peppol-cli/internal/config"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
)

const apiSettingsURL = "https://app.e-invoice.be/api-settings"

func newAuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authenticate with the e-invoice.be API",
		RunE:  runAuth,
	}

	cmd.AddCommand(newAuthStatusCmd())
	cmd.AddCommand(newAuthLogoutCmd())

	return cmd
}

func runAuth(cmd *cobra.Command, args []string) error {
	w := cmd.OutOrStdout()

	fmt.Fprintf(w, "Open the following URL to get your API key:\n\n  %s\n\n", apiSettingsURL)

	// Try to open browser, ignore errors (works fine without it).
	_ = browser.OpenURL(apiSettingsURL)

	fmt.Fprint(w, "Paste your API key: ")
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return &ExitError{Err: fmt.Errorf("reading input: %w", err), Code: 1}
	}
	apiKey := strings.TrimSpace(line)
	if apiKey == "" {
		return &ExitError{Err: fmt.Errorf("no API key provided"), Code: 1}
	}

	// Validate the key by calling GET /api/me/.
	if !flags.Quiet {
		fmt.Fprint(w, "Validating...")
	}
	c := client.NewClient(apiKey)
	tenant, err := c.GetMe()
	if err != nil {
		fmt.Fprintln(w)
		return &ExitError{Err: fmt.Errorf("validation failed: %w", err), Code: 2}
	}
	if !flags.Quiet {
		fmt.Fprintln(w, " OK")
	}

	// Determine workspace name: use flag override, else derive from tenant name.
	dir := mustConfigDir()
	wsName := flags.Workspace
	if wsName == "" {
		wsName = slugify(tenant.Name)
	}
	if wsName == "" {
		wsName = "default"
	}

	// Store the key in workspace-specific keyring.
	kr := config.NewFileKeyringForWorkspace(dir, wsName)
	if err := kr.Set(apiKey); err != nil {
		return &ExitError{Err: fmt.Errorf("storing credentials: %w", err), Code: 1}
	}

	// Update config with workspace.
	cfg, err := config.LoadFrom(dir)
	if err != nil {
		cfg = &config.Config{Workspaces: make(map[string]config.Workspace)}
	}
	cfg.Workspaces[wsName] = config.Workspace{Name: tenant.Name}
	cfg.ActiveWorkspace = wsName
	if err := config.SaveTo(dir, cfg); err != nil {
		return &ExitError{Err: fmt.Errorf("saving config: %w", err), Code: 1}
	}

	fmt.Fprintf(w, "Authenticated as %s (workspace: %s)\n", tenant.Name, wsName)
	return nil
}

func newAuthStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show current authentication status",
		RunE:  runAuthStatus,
	}
}

func runAuthStatus(cmd *cobra.Command, args []string) error {
	w := cmd.OutOrStdout()
	dir := mustConfigDir()

	cfg, err := config.LoadFrom(dir)
	if err != nil {
		cfg = &config.Config{Workspaces: make(map[string]config.Workspace)}
	}

	// Determine workspace.
	workspace := flags.Workspace
	if workspace == "" {
		workspace = cfg.ActiveWorkspace
	}

	// Resolve key for the workspace.
	var kr config.KeyringBackend
	if workspace != "" {
		kr = config.NewFileKeyringForWorkspace(dir, workspace)
	} else {
		kr = config.NewFileKeyring(dir)
	}
	key, err := config.ResolveAPIKey(kr)
	if err != nil {
		return &ExitError{Err: fmt.Errorf("reading credentials: %w", err), Code: 1}
	}

	if key == "" {
		fmt.Fprintln(w, "Not authenticated")
		return nil
	}

	if ws, ok := cfg.Workspaces[workspace]; ok {
		fmt.Fprintf(w, "Authenticated as %s (workspace: %s, key: %s)\n", ws.Name, workspace, client.MaskKey(key))
	} else {
		fmt.Fprintf(w, "Authenticated (key: %s)\n", client.MaskKey(key))
	}
	return nil
}

func newAuthLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove stored credentials",
		RunE:  runAuthLogout,
	}
}

func runAuthLogout(cmd *cobra.Command, args []string) error {
	w := cmd.OutOrStdout()
	dir := mustConfigDir()

	cfg, err := config.LoadFrom(dir)
	if err != nil {
		cfg = &config.Config{Workspaces: make(map[string]config.Workspace)}
	}

	// Determine workspace.
	workspace := flags.Workspace
	if workspace == "" {
		workspace = cfg.ActiveWorkspace
	}

	// Remove credentials for the workspace.
	var kr config.KeyringBackend
	if workspace != "" {
		kr = config.NewFileKeyringForWorkspace(dir, workspace)
	} else {
		kr = config.NewFileKeyring(dir)
	}
	if err := kr.Remove(); err != nil {
		return &ExitError{Err: fmt.Errorf("removing credentials: %w", err), Code: 1}
	}

	// Remove workspace from config.
	if workspace != "" {
		delete(cfg.Workspaces, workspace)
		if cfg.ActiveWorkspace == workspace {
			cfg.ActiveWorkspace = ""
		}
	}
	_ = config.SaveTo(dir, cfg)

	fmt.Fprintln(w, "Logged out successfully")
	return nil
}

// slugify converts a name to a lowercase, hyphenated slug suitable for use as a workspace name.
func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	prevDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash && b.Len() > 0 {
				b.WriteRune('-')
				prevDash = true
			}
		}
	}
	result := b.String()
	return strings.TrimRight(result, "-")
}
