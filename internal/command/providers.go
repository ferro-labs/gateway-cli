package command

import (
	"github.com/spf13/cobra"

	"github.com/ferro-labs/gateway-cli/internal/api"
	"github.com/ferro-labs/gateway-cli/internal/table"
)

// The merged row, its five columns and the dash-for-absent rendering live in
// internal/table (providers.go) because the console renders the same listing
// and cannot import this package. table.ProviderRow's json tags are this
// command's --format json|yaml contract, unchanged by the move.
func newProvidersCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "providers",
		Short: "Provider health, circuit state and model counts",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			d := deps(cmd)
			health, _, err := d.Client.Health(cmd.Context())
			if err != nil {
				return err
			}
			// Best effort: an unusable credential costs the status and message
			// columns, not the command.
			admin, adminErr := d.Client.AdminHealth(cmd.Context())
			if adminErr != nil {
				admin = nil
				// Only a refused credential is reported as one; a timeout or a
				// gateway error says so instead, so the reader is not sent to
				// check a credential that was never the problem.
				why := "admin health unavailable"
				if api.IsUnauthorized(adminErr) {
					why = "no admin credential accepted"
				}
				d.Printer.Warn("%s — status and message come from /health only (%v)", why, adminErr)
			}

			rows := table.MergeProviders(health, admin)
			if d.Printer.Format != FormatTable {
				return d.printStructured(rows)
			}
			d.Printer.Table(table.ProviderHeaders, table.ProviderRows(rows))
			return nil
		},
	}
}
