package table

import (
	"slices"
	"testing"

	"github.com/ferro-labs/gateway-cli/internal/api"
)

// The listing the merge exists for: /health names two providers, /admin/health
// knows about fewer of them, and neither endpoint alone fills the five columns.
var providerHealth = &api.HealthReport{Providers: []api.ProviderHealth{
	{Name: "openai", Status: "available", Circuit: "closed", Models: 1104},
	{Name: "anthropic", Status: "available", Circuit: "half_open", Models: 412},
}}

// The console used to iterate /admin/health's list alone, so a provider that
// endpoint omitted disappeared from the screen while the pipe still printed
// it. Every case below asserts the whole row set, not a count: the bug was a
// missing row, and a length check would have missed which one.
func TestMergeProvidersUnionsBothListings(t *testing.T) {
	for _, tc := range []struct {
		name  string
		admin *api.AdminHealth
		want  []ProviderRow
	}{
		{
			// Unauthenticated is a smaller answer, not an error: /health alone
			// still names every provider and its circuit, and no row carries a
			// message because only /admin/health serves one.
			name:  "no credential leaves /health answering alone",
			admin: nil,
			want: []ProviderRow{
				{Name: "openai", Status: "available", Circuit: "closed", Models: 1104},
				{Name: "anthropic", Status: "available", Circuit: "half_open", Models: 412},
			},
		},
		{
			// Where both endpoints know a provider the authenticated reading
			// wins — it is the one with a message — and /health's circuit is
			// grafted on because /admin/health does not report it.
			name: "admin's status, message and count win, /health's circuit is grafted on",
			admin: &api.AdminHealth{Providers: []api.AdminProviderHealth{
				{Name: "openai", Status: "healthy", Models: 1104},
				{Name: "anthropic", Status: "degraded", Models: 3, Message: "rate limited upstream"},
			}},
			want: []ProviderRow{
				{Name: "openai", Status: "healthy", Circuit: "closed", Models: 1104},
				{Name: "anthropic", Status: "degraded", Circuit: "half_open", Models: 3,
					Message: "rate limited upstream"},
			},
		},
		{
			// The divergence itself: one row from /admin/health, one that only
			// /health knows. Dropping the second is what the console did.
			name: "a provider only /health reports still gets a row",
			admin: &api.AdminHealth{Providers: []api.AdminProviderHealth{
				{Name: "openai", Status: "healthy", Models: 1104},
			}},
			want: []ProviderRow{
				{Name: "openai", Status: "healthy", Circuit: "closed", Models: 1104},
				{Name: "anthropic", Status: "available", Circuit: "half_open", Models: 412},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := MergeProviders(providerHealth, tc.admin); !slices.Equal(got, tc.want) {
				t.Fatalf("merge lost or reshaped a row:\n got %+v\nwant %+v", got, tc.want)
			}
		})
	}
}

// A dash is how absence renders: an empty cell collapses the column for
// anything splitting this table on whitespace, and reads as a blank status
// rather than as no status at all. MODELS is deliberately not dashed — zero
// models is a reading the gateway gave, not a value it withheld.
func TestProviderRowsDashEveryAbsentCell(t *testing.T) {
	got := ProviderRows([]ProviderRow{{Name: "openai"}, {Name: "anthropic", Status: "healthy",
		Circuit: "closed", Models: 412, Message: "recovering"}})
	want := [][]string{
		{"openai", "-", "-", "0", "-"},
		{"anthropic", "healthy", "closed", "412", "recovering"},
	}
	if len(got) != len(want) {
		t.Fatalf("want %d rows, got %d (%v)", len(want), len(got), got)
	}
	for i := range want {
		if !slices.Equal(got[i], want[i]) {
			t.Fatalf("row %d: got %q, want %q", i, got[i], want[i])
		}
	}
	if len(want[0]) != len(ProviderHeaders) {
		t.Fatalf("a row must have one cell per header: %d cells, %d headers", len(want[0]), len(ProviderHeaders))
	}
}
