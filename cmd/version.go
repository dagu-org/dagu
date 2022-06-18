/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/yohamta/dagu/internal/constants"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "dagu version",
	RunE: func(cmd *cobra.Command, args []string) error {
		if constants.Version != "" {
			fmt.Println(constants.Version)
			return nil
		}
		return fmt.Errorf("failed to get version")
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
