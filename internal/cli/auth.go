package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/e-invoice-be/peppol-cli/internal/client"
	"github.com/e-invoice-be/peppol-cli/internal/config"
)

const apiSettingsURL = "https://app.e-invoice.be/api-settings"

func newAuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "auth",
		Short:   "Authenticate with the e-invoice.be API",
		Example: "  peppol auth",
		RunE:    runAuth,
	}

	cmd.AddCommand(newAuthStatusCmd())
	cmd.AddCommand(newAuthLogoutCmd())

	return cmd
}

func runAuth(cmd *cobra.Command, args []string) error {
	w := cmd.OutOrStdout()

	// Load the config before prompting so a broken config.yaml aborts the
	// command without storing a key or overwriting the user's workspaces.
	dir := mustConfigDir()
	cfg, err := loadConfig(dir)
	if err != nil {
		return err
	}
	if flags.Workspace != "" {
		if err := checkWorkspaceName(flags.Workspace, cfg); err != nil {
			return err
		}
	}

	errOut := cmd.ErrOrStderr()
	fmt.Fprintf(errOut, "Open the following URL to get your API key:\n\n  %s\n\n", apiSettingsURL)

	// Only open a browser for an interactive user; when the key is piped in
	// (scripts, CI) there is nobody to look at it. Errors are ignored: the
	// URL is printed above.
	if isTerminal(cmd.InOrStdin()) {
		_ = openBrowser(apiSettingsURL)
	}

	apiKey, err := readAPIKey(cmd)
	if err != nil {
		return err
	}
	tenant, err := validateAPIKey(cmd, apiKey)
	if err != nil {
		return err
	}

	// Determine workspace name: use flag override, else derive from tenant name.
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
	cfg.Workspaces[wsName] = config.Workspace{Name: tenant.Name}
	cfg.ActiveWorkspace = wsName
	if err := config.SaveTo(dir, cfg); err != nil {
		return &ExitError{Err: fmt.Errorf("saving config: %w", err), Code: 1}
	}

	fmt.Fprintf(w, "Authenticated as %s (workspace: %s)\n", tenant.Name, wsName)
	return nil
}

// openBrowser opens a URL in the user's browser; a variable so tests can stub it.
var openBrowser = browser.OpenURL

// isTerminal reports whether r is a terminal.
func isTerminal(r io.Reader) bool {
	f, ok := r.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}

// readAPIKey prompts on stderr and reads an API key line from the command's
// stdin, so the prompt never mixes with the command's output.
func readAPIKey(cmd *cobra.Command) (string, error) {
	fmt.Fprint(cmd.ErrOrStderr(), "Paste your API key: ")
	line, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	if err != nil && (!errors.Is(err, io.EOF) || line == "") {
		return "", &ExitError{Err: fmt.Errorf("reading input: %w", err), Code: 1}
	}
	apiKey := strings.TrimSpace(line)
	if apiKey == "" {
		return "", &ExitError{Err: errors.New("no API key provided"), Code: 1}
	}
	return apiKey, nil
}

// validateAPIKey checks apiKey by calling GET /api/me/, reporting progress
// on stderr, and returns the key's tenant.
func validateAPIKey(cmd *cobra.Command, apiKey string) (*client.TenantPublic, error) {
	errOut := cmd.ErrOrStderr()
	if !flags.Quiet {
		fmt.Fprint(errOut, "Validating...")
	}
	c := client.NewClient(apiKey, clientOpts()...).WithContext(cmd.Context())
	tenant, err := c.GetMe()
	if err != nil {
		if !flags.Quiet {
			fmt.Fprintln(errOut)
		}
		return nil, &ExitError{Err: fmt.Errorf("validation failed: %w", err), Code: 2}
	}
	if !flags.Quiet {
		fmt.Fprintln(errOut, " OK")
	}
	return tenant, nil
}

func newAuthStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "status",
		Short:   "Show current authentication status",
		Example: "  peppol auth status",
		RunE:    runAuthStatus,
	}
}

func runAuthStatus(cmd *cobra.Command, args []string) error {
	w := cmd.OutOrStdout()
	dir := mustConfigDir()

	cfg, err := loadConfig(dir)
	if err != nil {
		return err
	}

	// Determine workspace.
	workspace := flags.Workspace
	if workspace == "" {
		workspace = cfg.ActiveWorkspace
	} else if err := checkWorkspaceName(workspace, cfg); err != nil {
		return err
	}

	// PEPPOL_API_KEY takes precedence over stored credentials (see
	// config.ResolveAPIKey), so report it as the source rather than the
	// workspace, which may belong to a different account.
	if envKey := os.Getenv("PEPPOL_API_KEY"); envKey != "" {
		fmt.Fprintf(w, "Authenticated via PEPPOL_API_KEY (key: %s)\n", client.MaskKey(envKey))
		if _, ok := cfg.Workspaces[workspace]; ok {
			fmt.Fprintf(w, "Note: PEPPOL_API_KEY overrides the stored credentials of workspace %q\n", workspace)
		}
		return nil
	}

	// Resolve key for the workspace.
	var kr config.KeyringBackend
	if workspace != "" {
		kr = config.NewFileKeyringForWorkspace(dir, workspace)
	} else {
		kr = config.NewFileKeyring(dir)
	}
	key, err := kr.Get()
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
		Use:     "logout",
		Short:   "Remove stored credentials",
		Example: "  peppol auth logout\n  peppol auth logout -w production",
		RunE:    runAuthLogout,
	}
}

func runAuthLogout(cmd *cobra.Command, args []string) error {
	w := cmd.OutOrStdout()
	dir := mustConfigDir()

	cfg, err := loadConfig(dir)
	if err != nil {
		return err
	}

	// Determine workspace.
	workspace := flags.Workspace
	if workspace == "" {
		workspace = cfg.ActiveWorkspace
	} else if err := checkWorkspaceName(workspace, cfg); err != nil {
		return err
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
	if err := config.SaveTo(dir, cfg); err != nil {
		return &ExitError{Err: fmt.Errorf("saving config: %w", err), Code: 1}
	}

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
	// Keep the slug within the workspace-name limit so it always passes
	// config.ValidateWorkspaceName.
	if len(result) > config.MaxWorkspaceNameLen {
		result = result[:config.MaxWorkspaceNameLen]
	}
	return strings.TrimRight(result, "-")
}
