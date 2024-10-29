// Copyright (C) 2024 The Dagu Authors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	// cfgFile parameter
	cfgFile string

	// rootCmd represents the base command when called without any subcommands
	rootCmd = &cobra.Command{
		Use:   "dagu",
		Short: "YAML-based DAG scheduling tool.",
		Long:  `YAML-based DAG scheduling tool.`,
	}
)

// Execute adds all child commands to the root command and sets flags
// appropriately. This is called by main.main(). It only needs to happen
// once to the rootCmd.
func Execute() error {
	return rootCmd.Execute()
}

func registerCommands() {
	rootCmd.AddCommand(startCmd())
	rootCmd.AddCommand(stopCmd())
	rootCmd.AddCommand(restartCmd())
	rootCmd.AddCommand(dryCmd())
	rootCmd.AddCommand(statusCmd())
	rootCmd.AddCommand(versionCmd())
	rootCmd.AddCommand(serverCmd())
	rootCmd.AddCommand(schedulerCmd())
	rootCmd.AddCommand(retryCmd())
	rootCmd.AddCommand(startAllCmd())
	rootCmd.AddCommand(dequeueCmd())
}

func init() {
	rootCmd.PersistentFlags().
		StringVar(
			&cfgFile, "config", "",
			"config file (default is $HOME/.config/dagu/admin.yaml)",
		)

	cobra.OnInitialize(initialize)

	registerCommands()
}

func initialize() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
		return
	}
}
