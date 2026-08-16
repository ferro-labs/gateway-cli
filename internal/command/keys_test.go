package command

import (
	"bytes"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ferro-labs/gateway-cli/internal/api"
	"github.com/ferro-labs/gateway-cli/internal/fixture"
)

// runInteractive is run() with the terminal faked and an answer already waiting
// on stdin. Faking is the only way in: isTTY asks real descriptors, and a test
// binary's stdin is never one of them.
func runInteractive(t *testing.T, srv *httptest.Server, answer string, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	t.Setenv("FERRO_API_KEY", "fgw_test")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	oldTTY, oldStdin := isTTY, stdinIsTTY
	isTTY = func(*os.File) bool { return true }
	stdinIsTTY = func() bool { return true }
	t.Cleanup(func() { isTTY, stdinIsTTY = oldTTY, oldStdin })

	root := NewRoot()
	var out, errb bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errb)
	root.SetIn(strings.NewReader(answer))
	root.SetArgs(append(args, "--gateway-url", srv.URL))
	err = root.Execute()
	return out.String(), errb.String(), err
}

// keyIsLive reports whether the fixture still serves id as an active key. The
// fixture's key store is stateful, so this is what tells an aborted revoke
// apart from one that ran and printed an error afterwards.
func keyIsLive(t *testing.T, srv *httptest.Server, id string) bool {
	t.Helper()
	stdout, _, err := run(t, srv, "keys", "get", id)
	if err != nil {
		t.Fatalf("keys get %s: %v", id, err)
	}
	var got map[string]any
	if err := decodeOne(stdout, &got); err != nil {
		t.Fatalf("keys get %s: %v (%q)", id, err, stdout)
	}
	return got["revoked_at"] == nil
}

func TestKeysListDerivesStateAndNeverPrintsASecret(t *testing.T) {
	srv := gateway(t, fixture.Default())
	stdout, _, err := run(t, srv, "keys", "list")
	if err != nil {
		t.Fatal(err)
	}
	// ID is here and not in the plan's column set on purpose: rotate and revoke
	// take an id, and a table listing that omits it cannot start that workflow.
	for _, col := range []string{"ID", "NAME", "KEY", "SCOPES", "EXPIRES", "LAST USED", "USES", "STATE"} {
		if !strings.Contains(stdout, col) {
			t.Fatalf("missing column %s in %q", col, stdout)
		}
	}
	// The gateway serves no state field; all three states are derived from
	// revoked_at / active, and the seeded store carries one of each.
	for _, state := range []string{"active", "revoked", "expired"} {
		if !strings.Contains(stdout, state) {
			t.Fatalf("derived STATE %q missing: %q", state, stdout)
		}
	}
	// The wire masks reads as head8...tail4; anything longer would mean the
	// listing had grown a full secret.
	for _, line := range strings.Split(stdout, "\n") {
		for _, f := range strings.Fields(line) {
			if strings.HasPrefix(f, "fgw_") && !strings.Contains(f, "...") {
				t.Fatalf("list printed an unmasked credential: %q", f)
			}
		}
	}
}

func TestKeysListJSONIsOneDocument(t *testing.T) {
	srv := gateway(t, fixture.Default())
	stdout, _, err := run(t, srv, "keys", "list", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []map[string]any
	if err := decodeOne(stdout, &rows); err != nil {
		t.Fatalf("stdout must be one JSON document: %v (%q)", err, stdout)
	}
	if len(rows) == 0 {
		t.Fatalf("expected the seeded keys: %q", stdout)
	}
}

func TestKeysCreateSecretOnStdoutWarningOnStderr(t *testing.T) {
	srv := gateway(t, fixture.Default())
	stdout, stderr, err := run(t, srv, "keys", "create", "--name", "ci", "--scope", "read_only", "--expires-in", "720h")
	if err != nil {
		t.Fatal(err)
	}
	secret := strings.TrimSpace(stdout)
	if !strings.HasPrefix(secret, "fgw_") || strings.Contains(secret, "...") {
		t.Fatalf("the full secret must go to stdout so it can be piped, got %q", stdout)
	}
	if !strings.Contains(stderr, "shown once") {
		t.Fatalf("the shown-once warning belongs on stderr, got %q", stderr)
	}
	if strings.Contains(stdout, "shown once") {
		t.Fatalf("narrative leaked into the pipeable channel: %q", stdout)
	}
}

func TestKeysCreateRejectsBadScopeBeforeAnyRequest(t *testing.T) {
	_, _, err := execute(t, "keys", "create", "--name", "x", "--scope", "root",
		"--gateway-url", "http://127.0.0.1:1")
	if err == nil || !strings.Contains(err.Error(), "admin|read_only") {
		t.Fatalf("scope must be validated client-side, naming the valid values: %v", err)
	}
	if !strings.Contains(err.Error(), "always sends one explicitly") {
		t.Fatalf("the error must explain why ferro always sends a scope: %v", err)
	}
	// The gateway defaults a scope-less create to read_only, so the message
	// must not claim the opposite — it said "granted admin server-side" while
	// defaultScopes was minting least privilege.
	if strings.Contains(err.Error(), "server-side") {
		t.Fatalf("the message must not restate the old backwards claim: %v", err)
	}
}

func TestKeysCreateRequiresName(t *testing.T) {
	_, _, err := execute(t, "keys", "create", "--gateway-url", "http://127.0.0.1:1")
	if err == nil || !strings.Contains(err.Error(), "--name") {
		t.Fatalf("a nameless key is unattributable later: %v", err)
	}
}

func TestKeysCreateRejectsNegativeExpiry(t *testing.T) {
	_, err := keyCreateRequest("ci", "read_only", -time.Hour)
	if err == nil || !strings.Contains(err.Error(), "positive duration") {
		t.Fatalf("negative expiry must not become a non-expiring key: %v", err)
	}
}

func TestKeysRotateShowsNewSecretOnce(t *testing.T) {
	srv := gateway(t, fixture.Default())
	stdout, stderr, err := run(t, srv, "keys", "rotate", "key_02", "--yes")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(strings.TrimSpace(stdout), "fgw_") {
		t.Fatalf("rotate must print the new secret to stdout: %q", stdout)
	}
	if !strings.Contains(stderr, "shown once") || !strings.Contains(stderr, "stopped authenticating") {
		t.Fatalf("stderr must say the old secret is dead and the new one is shown once: %q", stderr)
	}
}

func TestKeysRevokeConfirmsOnStderrAndPrintsNothingToStdout(t *testing.T) {
	srv := gateway(t, fixture.Default())
	stdout, stderr, err := run(t, srv, "keys", "revoke", "key_01", "--yes")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Fatalf("revoke has no payload; stdout must stay empty: %q", stdout)
	}
	if !strings.Contains(stderr, "key_01") {
		t.Fatalf("the confirmation belongs on stderr: %q", stderr)
	}
}

func TestKeysGetResolvesOneKey(t *testing.T) {
	srv := gateway(t, fixture.Default())
	stdout, _, err := run(t, srv, "keys", "get", "key_01")
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := decodeOne(stdout, &got); err != nil {
		t.Fatalf("keys get renders a structured document: %v (%q)", err, stdout)
	}
	if got["id"] != "key_01" {
		t.Fatalf("wrong key: %v", got)
	}
}

func TestKeysGetUnknownIDFails(t *testing.T) {
	srv := gateway(t, fixture.Default())
	if _, _, err := run(t, srv, "keys", "get", "key_nope"); err == nil {
		t.Fatal("an unknown key id must be an error, not an empty success")
	}
}

// The operator-facing symptom of the KeyState bug: a key whose expiry has
// passed is served active:true with revoked_at:null, and STATE showed
// "active" — the one answer that matters on a surface read to decide whether
// a credential still works.
func TestKeysListReportsAnExpiredKeyAsExpired(t *testing.T) {
	srv := gateway(t, fixture.Default())
	stdout, _, err := run(t, srv, "keys", "list")
	if err != nil {
		t.Fatal(err)
	}
	var row string
	for _, l := range strings.Split(stdout, "\n") {
		if strings.Contains(l, "demo-temp") {
			row = l
		}
	}
	if row == "" {
		t.Fatalf("expected the seeded expired key: %q", stdout)
	}
	if !strings.Contains(row, api.KeyStateExpired) {
		t.Fatalf("a key past its expiry must report %q, got %q", api.KeyStateExpired, row)
	}
	if strings.Contains(row, api.KeyStateRevoked) {
		t.Fatalf("an expired key was never revoked: %q", row)
	}
}

// The pipeline case: no terminal to prompt on, so the only two acceptable
// outcomes are --yes or a refusal. Hanging on a read nobody will answer would
// wedge a CI job, and revoking unasked is what this gate exists to stop.
func TestKeysDestructiveVerbsRefuseWithoutTerminalOrYes(t *testing.T) {
	for _, verb := range []string{"rotate", "revoke"} {
		t.Run(verb, func(t *testing.T) {
			srv := gateway(t, fixture.Default())
			stdout, _, err := run(t, srv, "keys", verb, "key_01")
			if err == nil {
				t.Fatal("a non-interactive destructive verb must refuse, not proceed")
			}
			if !strings.Contains(err.Error(), "--yes") {
				t.Fatalf("the refusal must name the way through it: %v", err)
			}
			if strings.TrimSpace(stdout) != "" {
				t.Fatalf("nothing ran, so stdout must be empty: %q", stdout)
			}
			if !keyIsLive(t, srv, "key_01") {
				t.Fatal("the refused verb reached the gateway anyway")
			}
		})
	}
}

func TestKeysRevokeProceedsWhenTheIDIsTypedBack(t *testing.T) {
	srv := gateway(t, fixture.Default())
	stdout, stderr, err := runInteractive(t, srv, "key_01\n", "keys", "revoke", "key_01")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr, "type the key id") {
		t.Fatalf("the prompt belongs on stderr: %q", stderr)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Fatalf("neither prompt nor result is a payload: %q", stdout)
	}
	if keyIsLive(t, srv, "key_01") {
		t.Fatal("a confirmed revoke must actually revoke")
	}
}

// "y" is exactly the answer this prompt refuses to accept: the console makes
// the operator type the key's name for the same reason.
func TestKeysRevokeAbortsOnAnythingButTheID(t *testing.T) {
	srv := gateway(t, fixture.Default())
	_, _, err := runInteractive(t, srv, "y\n", "keys", "revoke", "key_01")
	if err == nil || !strings.Contains(err.Error(), "aborted") {
		t.Fatalf("a wrong answer must abort: %v", err)
	}
	if !keyIsLive(t, srv, "key_01") {
		t.Fatal("an aborted revoke revoked the key anyway")
	}
}

// Ctrl-D at the prompt: an empty answer is a wrong answer, not a default yes.
func TestKeysRotateAbortsOnEOFAndMintsNoSecret(t *testing.T) {
	srv := gateway(t, fixture.Default())
	stdout, _, err := runInteractive(t, srv, "", "keys", "rotate", "key_02")
	if err == nil || !strings.Contains(err.Error(), "aborted") {
		t.Fatalf("EOF at the prompt must abort: %v", err)
	}
	if strings.Contains(stdout, "fgw_") {
		t.Fatalf("an aborted rotate must mint nothing: %q", stdout)
	}
}
