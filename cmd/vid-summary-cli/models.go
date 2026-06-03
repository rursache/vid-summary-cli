package main

import (
	"fmt"

	"github.com/rursache/vid-summary-cli/internal/model"
	"github.com/spf13/cobra"
)

func newModelsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "models",
		Short: "Manage whisper.cpp models",
	}

	list := &cobra.Command{
		Use:   "list",
		Short: "List local and known-available models",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), "Known models:")
			for _, name := range model.KnownNames() {
				_, exists, err := model.Local(name)
				if err != nil {
					return err
				}
				status := "not downloaded"
				if exists {
					status = "downloaded"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %-22s %s\n", name, status)
			}
			return nil
		},
	}

	download := &cobra.Command{
		Use:   "download <model>",
		Short: "Download a model to the local cache",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := model.Ensure(args[0])
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Ready: %s\n", path)
			return nil
		},
	}

	rm := &cobra.Command{
		Use:   "rm <model>",
		Short: "Remove a downloaded model",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := model.Remove(args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed %s\n", args[0])
			return nil
		},
	}

	cmd.AddCommand(list, download, rm)
	return cmd
}
