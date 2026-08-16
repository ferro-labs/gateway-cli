package command

import (
	"github.com/spf13/cobra"

	"github.com/ferro-labs/gateway-cli/internal/table"
)

func newModelsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "models",
		Short: "List the models this gateway routes",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			d := deps(cmd)
			models, err := d.Client.Models(cmd.Context())
			if err != nil {
				return err
			}
			if d.Printer.Format != FormatTable {
				return d.printStructured(models)
			}
			d.Printer.Table(table.ModelHeaders, table.ModelRows(models))
			return nil
		},
	}
}
