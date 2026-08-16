// Package version holds build metadata injected via -ldflags, mirroring the
// gateway's internal/version so release tooling stays uniform across the two
// projects. What the linker did not stamp is recovered from the build info the
// toolchain embeds anyway — see fillFromBuildInfo.
package version

import (
	"fmt"
	"runtime/debug"
)

// Version, Commit, and Date are overwritten at link time by the release build.
var (
	Version = devVersion
	Commit  = noCommit
	Date    = noDate
)

// The values a build with no -ldflags starts from. They double as the marker
// that the linker stamped nothing, which is what lets fillFromBuildInfo tell
// "not set" from "set to something that happens to look unhelpful".
const (
	devVersion = "dev"
	noCommit   = "none"
	noDate     = "unknown"
)

func init() { fillFromBuildInfo(debug.ReadBuildInfo) }

// fillFromBuildInfo recovers what the linker did not set.
//
// `go install github.com/ferro-labs/gateway-cli/cmd/ferro@v0.1.0` is the
// install path the README documents first, and go install applies no ldflags —
// so without this, everyone who followed the README reports
// "dev (commit none, built unknown)" and cannot tell a bug report apart from
// any other. The module version is in the build info for that build, and a
// build from a checkout carries the revision and time as vcs settings.
//
// A stamped field is never overwritten: what the release build says is what
// ferro reports, so a goreleaser binary and this fallback can never disagree.
// read is a parameter so a test can supply build info a test binary cannot have.
func fillFromBuildInfo(read func() (*debug.BuildInfo, bool)) {
	bi, ok := read()
	if !ok {
		return
	}
	// "(devel)" is what a local build reports for its own module; it says less
	// than "dev" already does and would only obscure that nothing stamped this.
	if Version == devVersion && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		Version = bi.Main.Version
	}
	// Read all three before deciding: the settings arrive in no promised order,
	// and vcs.modified changes what vcs.revision is allowed to claim.
	var revision, when string
	var dirty bool
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		case "vcs.time":
			when = s.Value
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	if Commit == noCommit && revision != "" {
		if dirty {
			// The worktree had uncommitted changes, so this commit does not
			// describe the binary. Say so rather than name a tree nobody can
			// check out to reproduce it.
			revision += "-dirty"
		}
		Commit = revision
	}
	if Date == noDate && when != "" {
		Date = when
	}
}

// String renders the full build stamp for `ferro version`.
func String() string {
	return fmt.Sprintf("%s (commit %s, built %s)", Version, Commit, Date)
}
