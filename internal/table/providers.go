package table

import (
	"strconv"

	"github.com/ferro-labs/gateway-cli/internal/api"
)

// ProviderHeaders and ProviderRows are the provider listing's shape, and
// MergeProviders is the reading they render, held in one place because two
// callers print it: `ferro providers` and the console's providers verb. They
// each carried their own copy of the merge, under near-identical comments
// claiming they did the same thing, and they did not: the console iterated
// /admin/health's list alone, so a provider /health reported and /admin/health
// did not was a row in the pipe and no row on screen — two answers for one
// gateway at one instant. The union here loses nothing, and is now the only one.
//
// This is the shape models.go and plugins.go already established: the wire
// types are internal/api's, the presentation of them is here, and neither
// caller owns it.
var ProviderHeaders = []string{"PROVIDER", "STATUS", "CIRCUIT", "MODELS", "MESSAGE"}

// ProviderRow is the merged view: no single endpoint serves all five columns.
// /health is unauthenticated and carries the circuit; /admin/health needs a
// credential and carries the status, the model count and the failure message.
//
// The json tags are `ferro providers --format json|yaml`'s output contract and
// are frozen — which is why the whole type moved here rather than the console
// growing a second one beside it. A dash is a rendering of absence, so it is
// applied in ProviderRows and never stored: these fields stay literally what
// the gateway said, empty string included.
type ProviderRow struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Circuit string `json:"circuit"`
	Models  int    `json:"models"`
	Message string `json:"message,omitempty"`
}

// MergeProviders unions the two listings, ah being nil when no credential
// worked — unauthenticated is a smaller answer, not an error.
//
// /admin/health's reading wins where both endpoints name a provider: it is the
// only one carrying a message, and its status vocabulary is the richer one.
// /health's circuit is grafted on because /admin/health does not report it.
// Providers only /health knows are appended rather than dropped: /admin/health
// listing fewer of them is an omission by that endpoint, not a provider that
// stopped existing.
func MergeProviders(h *api.HealthReport, ah *api.AdminHealth) []ProviderRow {
	circuits := make(map[string]string, len(h.Providers))
	for _, p := range h.Providers {
		circuits[p.Name] = p.Circuit
	}

	// No credential is just an empty authenticated listing, so both loops run
	// either way and the union has one implementation rather than two.
	var admin []api.AdminProviderHealth
	if ah != nil {
		admin = ah.Providers
	}
	rows := make([]ProviderRow, 0, max(len(admin), len(h.Providers)))
	seen := make(map[string]struct{}, len(admin))
	for _, p := range admin {
		seen[p.Name] = struct{}{}
		rows = append(rows, ProviderRow{
			Name: p.Name, Status: p.Status, Circuit: circuits[p.Name],
			Models: p.Models, Message: p.Message,
		})
	}
	for _, p := range h.Providers {
		if _, ok := seen[p.Name]; ok {
			continue
		}
		rows = append(rows, ProviderRow{
			Name: p.Name, Status: p.Status, Circuit: p.Circuit, Models: p.Models,
		})
	}
	return rows
}

// ProviderRows renders merged rows in ProviderHeaders' order. Every cell that
// can be absent goes through OrDash: an empty one collapses the column for
// anything splitting this table on whitespace, and reads as a blank status
// rather than as its absence. MODELS is the exception — a provider routing
// zero models is a reading the gateway gave, so it prints 0.
func ProviderRows(rows []ProviderRow) [][]string {
	cells := make([][]string, 0, len(rows))
	for _, r := range rows {
		cells = append(cells, []string{
			r.Name, OrDash(r.Status), OrDash(r.Circuit), strconv.Itoa(r.Models), OrDash(r.Message),
		})
	}
	return cells
}
