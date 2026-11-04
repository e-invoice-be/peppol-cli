package cli

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/e-invoice-be/peppol-cli/internal/client"
	"github.com/e-invoice-be/peppol-cli/internal/config"
	"github.com/e-invoice-be/peppol-cli/internal/output"
)

func newWorkspaceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "workspace",
		Aliases: []string{"ws"},
		Short:   "Manage workspaces",
		Example: "  peppol workspace list\n  peppol ws add production",
	}

	cmd.AddCommand(newWorkspaceAddCmd())
	cmd.AddCommand(newWorkspaceListCmd())
	cmd.AddCommand(newWorkspaceUseCmd())
	cmd.AddCommand(newWorkspaceRemoveCmd())

	return cmd
}

func newWorkspaceAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "add <name>",
		Short:   "Add a new workspace",
		Example: "  peppol workspace add production",
		Args:    cobra.ExactArgs(1),
		RunE:    runWorkspaceAdd,
	}
}

func runWorkspaceAdd(cmd *cobra.Command, args []string) error {
	name := args[0]
	w := cmd.OutOrStdout()
	dir := mustConfigDir()

	// New workspaces must always have a valid name.
	if err := checkWorkspaceName(name, nil); err != nil {
		return err
	}

	cfg, err := loadConfig(dir)
	if err != nil {
		return err
	}

	// Check if workspace already exists.
	if _, exists := cfg.Workspaces[name]; exists {
		return &ExitError{Err: fmt.Errorf("workspace %q already exists", name), Code: 1}
	}

	// Read API key from stdin.
	fmt.Fprint(w, "Paste your API key: ")
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return &ExitError{Err: fmt.Errorf("reading input: %w", err), Code: 1}
	}
	apiKey := strings.TrimSpace(line)
	if apiKey == "" {
		return &ExitError{Err: errors.New("no API key provided"), Code: 1}
	}

	// Validate the key.
	if !flags.Quiet {
		fmt.Fprint(w, "Validating...")
	}
	c := client.NewClient(apiKey, clientOpts()...).WithContext(cmd.Context())
	tenant, err := c.GetMe()
	if err != nil {
		fmt.Fprintln(w)
		return &ExitError{Err: fmt.Errorf("validation failed: %w", err), Code: 2}
	}
	if !flags.Quiet {
		fmt.Fprintln(w, " OK")
	}

	// Store key in workspace-specific keyring.
	kr := config.NewFileKeyringForWorkspace(dir, name)
	if err := kr.Set(apiKey); err != nil {
		return &ExitError{Err: fmt.Errorf("storing credentials: %w", err), Code: 1}
	}

	// Add workspace to config.
	if err := cfg.AddWorkspace(name, config.Workspace{Name: tenant.Name}); err != nil {
		return &ExitError{Err: err, Code: 1}
	}
	if err := config.SaveTo(dir, cfg); err != nil {
		return &ExitError{Err: fmt.Errorf("saving config: %w", err), Code: 1}
	}

	fmt.Fprintf(w, "Workspace %q added (tenant: %s)\n", name, tenant.Name)
	return nil
}

func newWorkspaceListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Short:   "List all workspaces",
		Example: "  peppol workspace list\n  peppol workspace list --json",
		Args:    cobra.NoArgs,
		RunE:    runWorkspaceList,
	}
}

func runWorkspaceList(cmd *cobra.Command, args []string) error {
	dir := mustConfigDir()
	cfg, err := loadConfig(dir)
	if err != nil {
		return err
	}

	r := output.FromContext(cmd.Context())

	if r.IsJSON() {
		type wsJSON struct {
			Name   string `json:"name"`
			Tenant string `json:"tenant"`
			Active bool   `json:"active"`
		}
		list := make([]wsJSON, 0, len(cfg.Workspaces))
		for _, name := range cfg.WorkspaceNames() {
			ws := cfg.Workspaces[name]
			list = append(list, wsJSON{
				Name:   name,
				Tenant: ws.Name,
				Active: name == cfg.ActiveWorkspace,
			})
		}
		return r.JSON(list)
	}

	if len(cfg.Workspaces) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No workspaces configured. Run 'peppol workspace add <name>' to add one.")
		return nil
	}

	headers := []string{"NAME", "TENANT", "ACTIVE"}
	var rows [][]string
	for _, name := range cfg.WorkspaceNames() {
		ws := cfg.Workspaces[name]
		active := ""
		if name == cfg.ActiveWorkspace {
			active = "*"
		}
		rows = append(rows, []string{name, ws.Name, active})
	}
	return r.Table(headers, rows)
}

func newWorkspaceUseCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "use <name>",
		Short:   "Switch active workspace",
		Example: "  peppol workspace use production",
		Args:    cobra.ExactArgs(1),
		RunE:    runWorkspaceUse,
	}
}

func runWorkspaceUse(cmd *cobra.Command, args []string) error {
	name := args[0]
	dir := mustConfigDir()

	cfg, err := loadConfig(dir)
	if err != nil {
		return err
	}
	if err := checkWorkspaceName(name, cfg); err != nil {
		return err
	}

	if err := cfg.SetActiveWorkspace(name); err != nil {
		return &ExitError{Err: err, Code: 1}
	}

	if err := config.SaveTo(dir, cfg); err != nil {
		return &ExitError{Err: fmt.Errorf("saving config: %w", err), Code: 1}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Switched to workspace %q\n", name)
	return nil
}

func newWorkspaceRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "remove <name>",
		Short:   "Remove a workspace",
		Example: "  peppol workspace remove staging",
		Args:    cobra.ExactArgs(1),
		RunE:    runWorkspaceRemove,
	}
}

func runWorkspaceRemove(cmd *cobra.Command, args []string) error {
	name := args[0]
	dir := mustConfigDir()

	cfg, err := loadConfig(dir)
	if err != nil {
		return err
	}
	if err := checkWorkspaceName(name, cfg); err != nil {
		return err
	}

	if err := cfg.RemoveWorkspace(name); err != nil {
		return &ExitError{Err: err, Code: 1}
	}

	// Remove credentials.
	kr := config.NewFileKeyringForWorkspace(dir, name)
	if err := kr.Remove(); err != nil {
		return &ExitError{Err: fmt.Errorf("removing credentials: %w", err), Code: 1}
	}

	if err := config.SaveTo(dir, cfg); err != nil {
		return &ExitError{Err: fmt.Errorf("saving config: %w", err), Code: 1}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Workspace %q removed\n", name)
	return nil
}

// loadConfig reads config.yaml from dir. A missing file yields an empty
// config; any other failure (unreadable or unparsable file) is returned as an
// exit-code-1 error naming the file, so callers never fall back to an empty
// config and overwrite the user's workspaces.
func loadConfig(dir string) (*config.Config, error) {
	cfg, err := config.LoadFrom(dir)
	if err != nil {
		path := filepath.Join(dir, "config.yaml")
		return nil, &ExitError{
			Err:  fmt.Errorf("loading config %s: %w (fix or move the file; nothing was changed)", path, err),
			Code: 1,
		}
	}
	return cfg, nil
}

// checkWorkspaceName validates a workspace name supplied by the user. A name
// that already exists in cfg is accepted as-is, so workspaces created before
// names were validated stay usable (the file keyring still refuses any name
// that would place credentials outside the config directory). Pass a nil cfg
// to validate a new name strictly.
func checkWorkspaceName(name string, cfg *config.Config) error {
	if cfg != nil {
		if _, ok := cfg.Workspaces[name]; ok {
			return nil
		}
	}
	if err := config.ValidateWorkspaceName(name); err != nil {
		return &ExitError{Err: err, Code: 1}
	}
	return nil
}
