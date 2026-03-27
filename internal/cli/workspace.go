package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/e-invoicebe/peppol-cli/internal/client"
	"github.com/e-invoicebe/peppol-cli/internal/config"
	"github.com/e-invoicebe/peppol-cli/internal/output"
	"github.com/spf13/cobra"
)

func newWorkspaceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "workspace",
		Aliases: []string{"ws"},
		Short:   "Manage workspaces",
	}

	cmd.AddCommand(newWorkspaceAddCmd())
	cmd.AddCommand(newWorkspaceListCmd())
	cmd.AddCommand(newWorkspaceUseCmd())
	cmd.AddCommand(newWorkspaceRemoveCmd())

	return cmd
}

func newWorkspaceAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <name>",
		Short: "Add a new workspace",
		Args:  cobra.ExactArgs(1),
		RunE:  runWorkspaceAdd,
	}
}

func runWorkspaceAdd(cmd *cobra.Command, args []string) error {
	name := args[0]
	w := cmd.OutOrStdout()
	dir := mustConfigDir()

	cfg, err := config.LoadFrom(dir)
	if err != nil {
		return &ExitError{Err: fmt.Errorf("loading config: %w", err), Code: 1}
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
		return &ExitError{Err: fmt.Errorf("no API key provided"), Code: 1}
	}

	// Validate the key.
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
		Use:   "list",
		Short: "List all workspaces",
		Args:  cobra.NoArgs,
		RunE:  runWorkspaceList,
	}
}

func runWorkspaceList(cmd *cobra.Command, args []string) error {
	dir := mustConfigDir()
	cfg, err := config.LoadFrom(dir)
	if err != nil {
		return &ExitError{Err: fmt.Errorf("loading config: %w", err), Code: 1}
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
		Use:   "use <name>",
		Short: "Switch active workspace",
		Args:  cobra.ExactArgs(1),
		RunE:  runWorkspaceUse,
	}
}

func runWorkspaceUse(cmd *cobra.Command, args []string) error {
	name := args[0]
	dir := mustConfigDir()

	cfg, err := config.LoadFrom(dir)
	if err != nil {
		return &ExitError{Err: fmt.Errorf("loading config: %w", err), Code: 1}
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
		Use:   "remove <name>",
		Short: "Remove a workspace",
		Args:  cobra.ExactArgs(1),
		RunE:  runWorkspaceRemove,
	}
}

func runWorkspaceRemove(cmd *cobra.Command, args []string) error {
	name := args[0]
	dir := mustConfigDir()

	cfg, err := config.LoadFrom(dir)
	if err != nil {
		return &ExitError{Err: fmt.Errorf("loading config: %w", err), Code: 1}
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
