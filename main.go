package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/selund/gv/internal/ui"
)

var (
	cfgFile    string
	targetPath string
	baseBranch string
)

var rootCmd = &cobra.Command{
	Use:   "gv [path]",
	Short: "Terminal UI diff viewer for agentic workflows",
	Long: `gv is a terminal UI for reviewing code changes across git worktrees.

Primary use case: monitoring multiple AI agents working in parallel,
each in their own worktree or branch.

Examples:
  gv                    # Run in current directory
  gv ~/projects/myapp   # Run in specific directory
  gv -b develop         # Compare against 'develop' branch
  gv -b main ~/myrepo   # Combine options`,
	Args: cobra.MaximumNArgs(1),
	RunE: run,
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.config/gv/config.yaml)")
	rootCmd.Flags().StringVarP(&baseBranch, "base", "b", "", "base branch to compare against (default: auto-detect main/master)")
	rootCmd.Flags().StringVarP(&targetPath, "path", "p", "", "path to repository (can also be positional arg)")

	viper.BindPFlag("base", rootCmd.Flags().Lookup("base"))
	viper.BindPFlag("path", rootCmd.Flags().Lookup("path"))
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err == nil {
			viper.AddConfigPath(filepath.Join(home, ".config", "gv"))
		}
		viper.AddConfigPath(".")
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}

	viper.SetEnvPrefix("GV")
	viper.AutomaticEnv()

	// Silently ignore missing config file
	viper.ReadInConfig()
}

func run(cmd *cobra.Command, args []string) error {
	// Determine path: positional arg > flag > cwd
	path := ""
	if len(args) > 0 {
		path = args[0]
	} else if viper.GetString("path") != "" {
		path = viper.GetString("path")
	}

	if path != "" {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("invalid path: %w", err)
		}
		if err := os.Chdir(absPath); err != nil {
			return fmt.Errorf("cannot access path: %w", err)
		}
	}

	// Build config
	cfg := ui.Config{
		BaseBranch: viper.GetString("base"),
	}

	model, err := ui.InitModelWithConfig(cfg)
	if err != nil {
		return err
	}

	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err = p.Run()
	return err
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
