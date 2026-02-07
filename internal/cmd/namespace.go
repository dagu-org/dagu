package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/core/spec"
	"github.com/spf13/cobra"
)

// CmdNamespace creates the namespace command with subcommands.
func CmdNamespace() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "namespace",
		Short: "Manage namespaces",
		Long: `Manage namespaces for isolating DAGs, runs, and configuration.

Available subcommands:
  list            - List all namespaces
  create          - Create a new namespace
  delete          - Delete a namespace
  set-base-config - Set namespace base configuration from a YAML file`,
	}

	cmd.AddCommand(NamespaceList())
	cmd.AddCommand(NamespaceCreate())
	cmd.AddCommand(NamespaceDelete())
	cmd.AddCommand(NamespaceSetBaseConfig())
	return cmd
}

// NamespaceList creates the namespace list subcommand.
func NamespaceList() *cobra.Command {
	return NewCommand(
		&cobra.Command{
			Use:   "list",
			Short: "List all namespaces",
			Long: `List all namespaces with their name, description, and creation date.

Output Formats:
  table (default) - Aligned table format
  json            - JSON array format

Examples:
  dagu namespace list
  dagu namespace list --format json`,
			Args: cobra.NoArgs,
		},
		namespaceListFlags,
		runNamespaceList,
	)
}

var namespaceListFlags = []commandLineFlag{
	namespaceListFormatFlag,
}

var namespaceListFormatFlag = commandLineFlag{
	name:         "format",
	shorthand:    "",
	defaultValue: "",
	usage:        "Output format: table (default) or json",
}

func runNamespaceList(ctx *Context, _ []string) error {
	format, err := ctx.StringParam("format")
	if err != nil {
		return fmt.Errorf("failed to get format parameter: %w", err)
	}

	if format != "" && format != "table" && format != "json" {
		return fmt.Errorf("invalid format %q: valid formats are table, json", format)
	}

	namespaces, err := ctx.NamespaceStore.List(ctx)
	if err != nil {
		return fmt.Errorf("failed to list namespaces: %w", err)
	}

	if len(namespaces) == 0 {
		fmt.Println("No namespaces found.")
		return nil
	}

	switch format {
	case "json":
		return renderNamespaceListJSON(namespaces)
	default:
		return renderNamespaceListTable(namespaces)
	}
}

func renderNamespaceListTable(namespaces []*exec.Namespace) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer func() {
		_ = w.Flush()
	}()

	if _, err := fmt.Fprintln(w, "NAME\tDESCRIPTION\tCREATED (UTC)"); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	for _, ns := range namespaces {
		description := ns.Description
		if description == "" {
			description = "-"
		}
		createdAt := ns.CreatedAt.UTC().Format("2006-01-02 15:04:05")

		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\n", ns.Name, description, createdAt); err != nil {
			return fmt.Errorf("failed to write row: %w", err)
		}
	}

	return nil
}

// NamespaceCreate creates the namespace create subcommand.
func NamespaceCreate() *cobra.Command {
	return NewCommand(
		&cobra.Command{
			Use:   "create <name>",
			Short: "Create a new namespace",
			Long: `Create a new namespace with the given name.

The name must match [a-z0-9][a-z0-9-]*[a-z0-9] and be at most 63 characters.

Examples:
  dagu namespace create team-alpha
  dagu namespace create team-beta --description "Beta team namespace"
  dagu namespace create team-gamma --default-queue my-queue --default-working-dir /tmp/work`,
			Args: cobra.ExactArgs(1),
		},
		namespaceCreateFlags,
		runNamespaceCreate,
	)
}

var namespaceCreateFlags = []commandLineFlag{
	namespaceCreateDescriptionFlag,
	namespaceCreateDefaultQueueFlag,
	namespaceCreateDefaultWorkingDirFlag,
}

var namespaceCreateDescriptionFlag = commandLineFlag{
	name:  "description",
	usage: "Description of the namespace",
}

var namespaceCreateDefaultQueueFlag = commandLineFlag{
	name:  "default-queue",
	usage: "Default queue name for DAGs in this namespace",
}

var namespaceCreateDefaultWorkingDirFlag = commandLineFlag{
	name:  "default-working-dir",
	usage: "Default working directory for DAGs in this namespace",
}

func runNamespaceCreate(ctx *Context, args []string) error {
	name := args[0]

	description, err := ctx.StringParam("description")
	if err != nil {
		return fmt.Errorf("failed to get description parameter: %w", err)
	}

	defaultQueue, err := ctx.StringParam("default-queue")
	if err != nil {
		return fmt.Errorf("failed to get default-queue parameter: %w", err)
	}

	defaultWorkingDir, err := ctx.StringParam("default-working-dir")
	if err != nil {
		return fmt.Errorf("failed to get default-working-dir parameter: %w", err)
	}

	ns, err := ctx.NamespaceStore.Create(ctx, exec.CreateNamespaceOptions{
		Name:        name,
		Description: description,
		Defaults: exec.NamespaceDefaults{
			Queue:      defaultQueue,
			WorkingDir: defaultWorkingDir,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create namespace: %w", err)
	}

	fmt.Printf("Namespace %q created (short ID: %s)\n", ns.Name, ns.ShortID)
	return nil
}

// NamespaceDelete creates the namespace delete subcommand.
func NamespaceDelete() *cobra.Command {
	return NewCommand(
		&cobra.Command{
			Use:   "delete <name>",
			Short: "Delete a namespace",
			Long: `Delete a namespace by name.

The namespace must not contain any DAGs. The "default" namespace cannot be deleted.

Examples:
  dagu namespace delete team-alpha
  dagu namespace delete team-beta --yes`,
			Args: cobra.ExactArgs(1),
		},
		namespaceDeleteFlags,
		runNamespaceDelete,
	)
}

var namespaceDeleteFlags = []commandLineFlag{
	yesFlag,
}

func runNamespaceDelete(ctx *Context, args []string) error {
	name := args[0]

	if name == "default" {
		return fmt.Errorf("cannot delete the \"default\" namespace")
	}

	// Look up the namespace to get its short ID.
	ns, err := ctx.NamespaceStore.Get(ctx, name)
	if err != nil {
		return fmt.Errorf("failed to get namespace %q: %w", name, err)
	}

	// Check if the namespace contains DAGs.
	dagDir := filepath.Join(ctx.Config.Paths.DAGsDir, ns.ShortID)
	if hasDAGs, checkErr := exec.NamespaceHasDAGs(dagDir); checkErr != nil {
		return fmt.Errorf("failed to check DAGs in namespace %q: %w", name, checkErr)
	} else if hasDAGs {
		return fmt.Errorf("namespace %q contains DAGs; remove all DAGs before deleting the namespace", name)
	}

	// Confirmation prompt (unless --yes).
	skipConfirm, _ := ctx.Command.Flags().GetBool("yes")
	if !skipConfirm {
		if !confirmAction(fmt.Sprintf("Delete namespace %q?", name)) {
			fmt.Println("Deletion cancelled.")
			return nil
		}
	}

	if err := ctx.NamespaceStore.Delete(ctx, name); err != nil {
		return fmt.Errorf("failed to delete namespace %q: %w", name, err)
	}

	fmt.Printf("Namespace %q deleted.\n", name)
	return nil
}

// NamespaceSetBaseConfig creates the namespace set-base-config subcommand.
func NamespaceSetBaseConfig() *cobra.Command {
	return NewCommand(
		&cobra.Command{
			Use:   "set-base-config <name>",
			Short: "Set namespace base configuration from a YAML file",
			Long: `Set the base configuration for a namespace by parsing a YAML file.

The YAML file should contain valid DAG configuration fields (env, logDir,
handlerOn, histRetentionDays, etc.) that will be applied as defaults to
all DAGs in the namespace.

Examples:
  dagu namespace set-base-config team-alpha --from-file base.yaml
  dagu namespace set-base-config default --from-file /etc/dagu/base-config.yaml`,
			Args: cobra.ExactArgs(1),
		},
		namespaceSetBaseConfigFlags,
		runNamespaceSetBaseConfig,
	)
}

var namespaceSetBaseConfigFlags = []commandLineFlag{
	namespaceSetBaseConfigFromFileFlag,
}

var namespaceSetBaseConfigFromFileFlag = commandLineFlag{
	name:     "from-file",
	usage:    "Path to the YAML file containing the base configuration",
	required: true,
}

func runNamespaceSetBaseConfig(ctx *Context, args []string) error {
	name := args[0]

	fromFile, err := ctx.StringParam("from-file")
	if err != nil {
		return fmt.Errorf("failed to get from-file parameter: %w", err)
	}

	if fromFile == "" {
		return fmt.Errorf("--from-file flag is required")
	}

	// Verify the namespace exists.
	if _, err := ctx.NamespaceStore.Get(ctx, name); err != nil {
		return fmt.Errorf("failed to get namespace %q: %w", name, err)
	}

	// Read and validate the YAML file.
	data, err := os.ReadFile(fromFile) // #nosec G304 - user-provided path for config file
	if err != nil {
		return fmt.Errorf("failed to read file %q: %w", fromFile, err)
	}

	// Parse the YAML as a DAG configuration to validate it.
	dag, err := spec.LoadYAML(context.Background(), data, spec.WithoutEval())
	if err != nil {
		return fmt.Errorf("invalid YAML configuration: %w", err)
	}

	// Update the namespace with the parsed base config and raw YAML.
	yamlStr := string(data)
	if _, err := ctx.NamespaceStore.Update(ctx, name, exec.UpdateNamespaceOptions{
		BaseConfig:     dag,
		BaseConfigYAML: &yamlStr,
	}); err != nil {
		return fmt.Errorf("failed to update namespace %q: %w", name, err)
	}

	fmt.Printf("Base configuration for namespace %q updated from %q.\n", name, fromFile)
	return nil
}

func renderNamespaceListJSON(namespaces []*exec.Namespace) error {
	type namespaceEntry struct {
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
		CreatedAt   string `json:"createdAt"`
	}

	entries := make([]namespaceEntry, 0, len(namespaces))
	for _, ns := range namespaces {
		entries = append(entries, namespaceEntry{
			Name:        ns.Name,
			Description: ns.Description,
			CreatedAt:   ns.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		})
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(entries)
}
