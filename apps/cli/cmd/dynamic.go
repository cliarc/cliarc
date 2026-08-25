package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/cliarc/cliarc/core/events"
	"github.com/cliarc/cliarc/core/permissions"
	manager "github.com/cliarc/cliarc/core/plugin-manager"
	"github.com/cliarc/cliarc/core/registry"
	"github.com/cliarc/cliarc/internal/manifest"
	pb "github.com/cliarc/cliarc/protocol/generated/go/cliarc/protocol"
)

// ReservedCoreNamespaces contains command names reserved strictly for CLIARC Core.
var ReservedCoreNamespaces = map[string]bool{
	"version":    true,
	"help":       true,
	"doctor":     true,
	"update":     true,
	"config":     true,
	"completion": true,
	"plugin":     true,
	"dev":        true,
	"server":     true,
	"use":        true,
	"create":     true,
}

// RegisterDynamicPluginCommands discovers enabled plugins and dynamically attaches their command trees to root.
func RegisterDynamicPluginCommands(root *cobra.Command) {
	home, _ := os.UserHomeDir()
	pluginDir := filepath.Join(home, ".cliarc", "plugins")

	stateStore, _ := manager.NewPluginStateStore("")

	// Collect candidate plugin directories
	candidateDirs := []string{pluginDir}
	for _, fb := range []string{"../plugins", "plugins", "./plugins"} {
		if stat, err := os.Stat(fb); err == nil && stat.IsDir() {
			candidateDirs = append(candidateDirs, fb)
		}
	}

	seenPlugins := make(map[string]bool)

	for _, cDir := range candidateDirs {
		entries, err := os.ReadDir(cDir)
		if err != nil {
			continue
		}

		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			pDir := filepath.Join(cDir, e.Name())
			mfPath, ok := manifest.FindManifestInDir(pDir)
			if !ok {
				continue
			}

			mf, err := manifest.Load(mfPath)
			if err != nil {
				continue
			}

			pName := strings.ToLower(mf.Name)
			if seenPlugins[pName] {
				continue
			}
			seenPlugins[pName] = true

			// Check if plugin is explicitly disabled
			if stateStore != nil {
				if rec, ok := stateStore.Get(pName); ok && !rec.Enabled {
					continue
				}
			}

			// Core namespace collision guard
			if ReservedCoreNamespaces[pName] {
				fmt.Fprintf(os.Stderr, color.YellowString("⚠️  Plugin namespace collision: plugin %q matches a reserved Core command and was skipped.\n", pName))
				continue
			}

			// Build dynamic Cobra command tree for this plugin
			pluginCmd := buildPluginCommandTree(mf)
			root.AddCommand(pluginCmd)
		}
	}
}

func buildPluginCommandTree(mf *manifest.Manifest) *cobra.Command {
	useName := strings.ToLower(mf.Name)
	if mf.ID != "" {
		useName = strings.ToLower(mf.ID)
	}

	var aliases []string
	if strings.ToLower(mf.Name) != useName {
		aliases = append(aliases, strings.ToLower(mf.Name))
	}
	if mf.Name != useName {
		aliases = append(aliases, mf.Name)
	}

	cmd := &cobra.Command{
		Use:     useName,
		Aliases: aliases,
		Short:   mf.Description,
		Long: fmt.Sprintf("%s\n\nPlugin: %s (v%s)\nAuthor: %s\nLicense: %s",
			mf.Description, mf.Name, mf.Version, mf.Author, mf.License),
		SilenceUsage: true,
	}

	// Attach subcommands from manifest CommandTree
	if len(mf.CommandTree) > 0 {
		for _, subDef := range mf.CommandTree {
			subCmd := buildSubcommand(mf, subDef)
			cmd.AddCommand(subCmd)
		}
	} else {
		// Fallback for flat actions
		for _, act := range mf.Actions {
			actName := act
			if strings.HasPrefix(act, mf.Name+".") {
				actName = strings.TrimPrefix(act, mf.Name+".")
			}
			actionCmd := &cobra.Command{
				Use:   actName,
				Short: fmt.Sprintf("Execute %s", act),
				RunE: func(c *cobra.Command, args []string) error {
					return executePluginAction(mf, act, args, c)
				},
			}
			cmd.AddCommand(actionCmd)
		}
	}

	return cmd
}

func buildSubcommand(mf *manifest.Manifest, def manifest.CommandDefinition) *cobra.Command {
	useStr := def.Name
	if def.Usage != "" {
		useStr = def.Usage
	}

	subCmd := &cobra.Command{
		Use:     useStr,
		Aliases: def.Aliases,
		Short:   def.Description,
		Long:    def.Description,
		RunE: func(c *cobra.Command, args []string) error {
			action := def.Handler
			if action == "" {
				action = def.Action
			}
			if action == "" {
				action = mf.Name + "." + def.Name
			}
			return executePluginAction(mf, action, args, c)
		},
	}

	// Register flags
	for _, flagDef := range def.Flags {
		switch strings.ToLower(flagDef.Type) {
		case "bool", "boolean":
			defVal := false
			if v, ok := flagDef.DefaultValue.(bool); ok {
				defVal = v
			}
			if flagDef.Shorthand != "" {
				subCmd.Flags().BoolP(flagDef.Name, flagDef.Shorthand, defVal, flagDef.Description)
			} else {
				subCmd.Flags().Bool(flagDef.Name, defVal, flagDef.Description)
			}
		case "int", "integer":
			defVal := 0
			if v, ok := flagDef.DefaultValue.(int); ok {
				defVal = v
			}
			if flagDef.Shorthand != "" {
				subCmd.Flags().IntP(flagDef.Name, flagDef.Shorthand, defVal, flagDef.Description)
			} else {
				subCmd.Flags().Int(flagDef.Name, defVal, flagDef.Description)
			}
		default:
			defVal := ""
			if v, ok := flagDef.DefaultValue.(string); ok {
				defVal = v
			}
			if flagDef.Shorthand != "" {
				subCmd.Flags().StringP(flagDef.Name, flagDef.Shorthand, defVal, flagDef.Description)
			} else {
				subCmd.Flags().String(flagDef.Name, defVal, flagDef.Description)
			}
		}

		if flagDef.Required {
			_ = subCmd.MarkFlagRequired(flagDef.Name)
		}
	}

	// Recursively attach nested subcommands
	for _, nested := range def.Subcommands {
		child := buildSubcommand(mf, nested)
		subCmd.AddCommand(child)
	}

	return subCmd
}

func executePluginAction(mf *manifest.Manifest, action string, args []string, cmd *cobra.Command) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	reg := registry.New()
	validator := permissions.NewValidator()
	bus := events.NewBus()

	mgr := manager.NewManager(reg, validator, bus, mf.Dir)

	if err := mgr.Load(mf); err != nil {
		return fmt.Errorf("failed to load plugin %q: %w", mf.Name, err)
	}

	if err := mgr.Start(ctx, mf.Name); err != nil {
		fmt.Fprintf(os.Stderr, color.RedString("❌ %s Plugin failed to load.\n\nReason:\n  %v\n\nSuggested action:\n  Run 'cliarc doctor' or check plugin dependencies.\n", strings.Title(mf.Name), err))
		return err
	}
	defer mgr.Stop(context.Background(), mf.Name)

	// Build payload from args, flags, and isolated config
	payload := map[string]interface{}{
		"args":   args,
		"action": action,
	}

	// Load isolated plugin config if exists
	home, _ := os.UserHomeDir()
	pluginConfigPath := filepath.Join(home, ".cliarc", "plugins", mf.Name, "config.json")
	if cfgData, err := os.ReadFile(pluginConfigPath); err == nil {
		var pCfg map[string]interface{}
		if err := json.Unmarshal(cfgData, &pCfg); err == nil {
			payload["config"] = pCfg
		}
	}

	// Add flags to payload
	if cmd != nil {
		cmd.Flags().VisitAll(func(f *pflag.Flag) {
			if f.Changed {
				payload[f.Name] = f.Value.String()
			}
		})
	}

	payloadBytes, _ := json.Marshal(payload)

	resp, err := mgr.Execute(ctx, mf.Name, action, payloadBytes)
	if err != nil {
		return fmt.Errorf("execution error: %w", err)
	}

	if resp.Status == pb.Status_STATUS_ERROR {
		if resp.Error != nil {
			return fmt.Errorf("[%s] %s", resp.Error.Code, resp.Error.Message)
		}
		return fmt.Errorf("plugin returned error status")
	}

	if len(resp.Result) > 0 {
		var prettyJSON bytes.Buffer
		if err := json.Indent(&prettyJSON, resp.Result, "", "  "); err == nil {
			fmt.Println(prettyJSON.String())
		} else {
			fmt.Println(string(resp.Result))
		}
	} else {
		fmt.Println(color.GreenString("✓ Action %q completed successfully", action))
	}

	return nil
}
