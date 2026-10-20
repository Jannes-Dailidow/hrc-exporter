package cli

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	"github.com/jannes-dailidow/hrc-exporter/internal/exporter"
	"github.com/jannes-dailidow/hrc-exporter/internal/i18n"
	"github.com/jannes-dailidow/hrc-exporter/internal/ui"
)

func (a *app) downloadCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "download",
		Aliases: []string{"dl", "sync"},
		Short:   a.p.T(i18n.CmdDownloadShort),
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()
			s, err := a.login(ctx)
			if err != nil {
				return err
			}
			defer s.close(ctx)

			opts := exporter.SyncOptions{Force: a.v.GetBool(keyForce), DryRun: a.v.GetBool(keyDryRun)}
			var sum exporter.Summary
			err = s.run(cancel, func(rep exporter.Reporter) error {
				var err error
				sum, err = s.exp.Sync(ctx, a.selection(), opts, rep)
				return err
			})
			ui.Renderer{P: a.p}.Summary(cmd.OutOrStdout(), sum)
			if err != nil {
				return err
			}
			if n := len(sum.Failures); n > 0 {
				return errors.New(a.p.N(i18n.ErrDownloadsFailed, n, n))
			}
			return nil
		},
	}
	f := cmd.Flags()
	f.Bool(keyForce, false, a.p.T(i18n.FlagForce))
	f.Bool(keyDryRun, false, a.p.T(i18n.FlagDryRun))
	_ = a.v.BindPFlags(f)
	return cmd
}

func (a *app) listCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   a.p.T(i18n.CmdListShort),
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()
			s, err := a.login(ctx)
			if err != nil {
				return err
			}
			defer s.close(ctx)

			var docs []exporter.DocumentStatus
			err = s.run(cancel, func(rep exporter.Reporter) error {
				var err error
				docs, err = s.exp.Status(ctx, a.selection(), rep)
				return err
			})
			if err != nil {
				return err
			}
			ui.Renderer{P: a.p}.Status(cmd.OutOrStdout(), docs)
			return nil
		},
	}
}
