package table

import (
	"strconv"

	"github.com/ferro-labs/gateway-cli/internal/api"
)

// ModelHeaders and ModelRows are the model listing's shape, held in one place
// because two callers render it: `ferro models` and the console's models verb.
// They were separate copies of the same six columns and the same six
// accessors, which is a drift waiting to happen — a column added to one reads
// as a column missing from the other, and the console's whole contract is that
// it shows what the scriptable verb shows.
//
// This is the shape NewPluginCatalog already established: the wire type is
// internal/api's, the presentation of it is here, and neither caller owns it.
var ModelHeaders = []string{"ID", "OWNED BY", "MODE", "CONTEXT", "CAPABILITIES", "STATUS"}

// ModelRows renders models in ModelHeaders' order. Capabilities is a count,
// not a list: the names are unbounded and would push STATUS off a narrow
// terminal, and the count is what a listing is scanned for.
func ModelRows(models []api.Model) [][]string {
	rows := make([][]string, 0, len(models))
	for _, m := range models {
		rows = append(rows, []string{
			m.ID,
			OrDash(m.OwnedBy),
			OrDash(m.Mode),
			PositiveOrDash(m.ContextWindow),
			strconv.Itoa(len(m.Capabilities)),
			OrDash(m.Status),
		})
	}
	return rows
}
