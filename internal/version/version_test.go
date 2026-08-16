package version

import (
	"runtime/debug"
	"strings"
	"testing"
)

// reset returns the package vars to their unstamped defaults for the duration
// of one test. They are package state written by init(), and this test binary's
// own build info has already run through it.
func reset(t *testing.T) {
	t.Helper()
	old := [3]string{Version, Commit, Date}
	t.Cleanup(func() { Version, Commit, Date = old[0], old[1], old[2] })
	Version, Commit, Date = devVersion, noCommit, noDate
}

func buildInfo(mainVersion string, settings ...debug.BuildSetting) func() (*debug.BuildInfo, bool) {
	return func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{Main: debug.Module{Version: mainVersion}, Settings: settings}, true
	}
}

// The release build is the one that must never be second-guessed: goreleaser
// stamps a tag and a commit, and a binary built from a checkout also carries
// vcs settings that disagree with them.
func TestLdflagsValuesWin(t *testing.T) {
	reset(t)
	Version, Commit, Date = "1.2.3", "abc1234", "2026-08-16T00:00:00Z"
	fillFromBuildInfo(buildInfo("v9.9.9",
		debug.BuildSetting{Key: "vcs.revision", Value: "deadbeef"},
		debug.BuildSetting{Key: "vcs.time", Value: "1999-01-01T00:00:00Z"}))
	if got := String(); got != "1.2.3 (commit abc1234, built 2026-08-16T00:00:00Z)" {
		t.Fatalf("the linker's values must survive the fallback, got %q", got)
	}
}

// The README's primary install path: go install applies no ldflags, but the
// module version it resolved is in the build info.
func TestGoInstallVersionFillsTheDevDefault(t *testing.T) {
	reset(t)
	fillFromBuildInfo(buildInfo("v0.1.0"))
	if Version != "v0.1.0" {
		t.Fatalf("go install must not report %q", Version)
	}
	// go install builds from the module cache, which has no VCS to read.
	if Commit != noCommit || Date != noDate {
		t.Fatalf("nothing supplied a commit or date: %q %q", Commit, Date)
	}
}

func TestVCSSettingsFillCommitAndDate(t *testing.T) {
	reset(t)
	fillFromBuildInfo(buildInfo("(devel)",
		debug.BuildSetting{Key: "vcs.revision", Value: "deadbeef"},
		debug.BuildSetting{Key: "vcs.time", Value: "2026-08-16T12:00:00Z"}))
	// "(devel)" carries no more information than the default it would replace.
	if Version != devVersion {
		t.Fatalf("(devel) must not become the reported version: %q", Version)
	}
	if Commit != "deadbeef" || Date != "2026-08-16T12:00:00Z" {
		t.Fatalf("vcs settings ignored: %q %q", Commit, Date)
	}
}

// A commit from a dirty worktree does not describe the binary, and a bug report
// quoting it would send somebody to the wrong tree.
func TestDirtyWorktreeIsMarked(t *testing.T) {
	reset(t)
	fillFromBuildInfo(buildInfo("",
		debug.BuildSetting{Key: "vcs.modified", Value: "true"},
		debug.BuildSetting{Key: "vcs.revision", Value: "deadbeef"}))
	if Commit != "deadbeef-dirty" {
		t.Fatalf("an uncommitted build must say so: %q", Commit)
	}
}

// `go run .` in a sandbox with no VCS and no ldflags. The stamp is useless but
// it still has to be a line, not three gaps in a sentence.
func TestNoLdflagsAndNoBuildInfoStillRenders(t *testing.T) {
	reset(t)
	fillFromBuildInfo(func() (*debug.BuildInfo, bool) { return nil, false })
	got := String()
	if got != "dev (commit none, built unknown)" {
		t.Fatalf("unstamped builds have a fixed rendering, got %q", got)
	}
	for _, part := range strings.Fields(got) {
		if part == "" || part == "()" {
			t.Fatalf("no field may render empty: %q", got)
		}
	}
}
