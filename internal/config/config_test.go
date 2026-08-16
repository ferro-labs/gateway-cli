package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestResolvePrecedence(t *testing.T) {
	file := File{
		CurrentProfile: "prod",
		Profiles: []Profile{
			{Name: "prod", URL: "https://gw.example.com", APIKeyEnv: "PROD_KEY"},
			{Name: "local", URL: "http://localhost:9999"},
		},
	}
	cases := []struct {
		name             string
		flagURL          string
		flagProfile      string
		envv             map[string]string
		wantURL, wantKey string
		wantSource       string
	}{
		{name: "URL flag and key environment beat profile", flagURL: "http://flag:1",
			envv:    map[string]string{"FERRO_URL": "http://env:1", "FERRO_API_KEY": "fgw_env"},
			wantURL: "http://flag:1", wantKey: "fgw_env", wantSource: "env:FERRO_API_KEY"},
		{name: "env beats profile",
			envv:    map[string]string{"FERRO_URL": "http://env:1", "FERRO_API_KEY": "fgw_env", "PROD_KEY": "fgw_prod"},
			wantURL: "http://env:1", wantKey: "fgw_env", wantSource: "env:FERRO_API_KEY"},
		{name: "profile url + key env deref",
			envv:    map[string]string{"PROD_KEY": "fgw_prod"},
			wantURL: "https://gw.example.com", wantKey: "fgw_prod", wantSource: "profile:PROD_KEY"},
		{name: "MASTER_KEY is the last key fallback",
			flagProfile: "local", envv: map[string]string{"MASTER_KEY": "fgw_master"},
			wantURL: "http://localhost:9999", wantKey: "fgw_master", wantSource: "env:MASTER_KEY"},
		// The whole point of the restriction: MASTER_KEY is in the shell on a
		// gateway host, and this invocation is aimed at somebody else's host.
		{name: "MASTER_KEY is withheld from a remote gateway", flagURL: "https://someone-else.example.com",
			flagProfile: "local", envv: map[string]string{"MASTER_KEY": "fgw_master"},
			wantURL: "https://someone-else.example.com", wantKey: "",
			wantSource: "env:MASTER_KEY skipped (gateway is not loopback)"},
		// A named credential is named for this tool, so the restriction must
		// not spread to it: the two upper steps still reach any host.
		{name: "a named credential still reaches a remote gateway", flagURL: "https://someone-else.example.com",
			envv:    map[string]string{"FERRO_API_KEY": "fgw_env", "MASTER_KEY": "fgw_master"},
			wantURL: "https://someone-else.example.com", wantKey: "fgw_env", wantSource: "env:FERRO_API_KEY"},
		{name: "defaults", flagProfile: "none-selected", envv: map[string]string{},
			flagURL: "", wantURL: "http://localhost:8080", wantKey: "", wantSource: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := file
			if tc.flagProfile == "none-selected" {
				f = File{} // no profiles at all
				tc.flagProfile = ""
			}
			r, err := Resolve(f, tc.flagURL, tc.flagProfile, env(tc.envv))
			if err != nil {
				t.Fatal(err)
			}
			if r.URL != tc.wantURL || r.APIKey != tc.wantKey || r.KeySource != tc.wantSource {
				t.Fatalf("got url=%q key=%q src=%q", r.URL, r.APIKey, r.KeySource)
			}
		})
	}
}

func TestResolveUnknownProfileErrors(t *testing.T) {
	_, err := Resolve(File{Profiles: []Profile{{Name: "a"}}}, "", "nope", env(nil))
	if err == nil {
		t.Fatal("unknown --profile must error and list valid names")
	}
}

func TestResolveStaleCurrentProfileErrors(t *testing.T) {
	_, err := Resolve(File{CurrentProfile: "removed", Profiles: []Profile{{Name: "prod"}}}, "", "", env(nil))
	if err == nil || !strings.Contains(err.Error(), `unknown profile "removed"`) {
		t.Fatalf("a stale current_profile must not silently connect to localhost: %v", err)
	}
	_, err = Resolve(File{CurrentProfile: "removed"}, "", "", env(nil))
	if err == nil || !strings.Contains(err.Error(), "no profiles are defined") {
		t.Fatalf("an empty profile list needs a clear error: %v", err)
	}
}

func TestLoadMissingFileIsZero(t *testing.T) {
	f, err := Load("/definitely/not/here.yaml")
	if err != nil || len(f.Profiles) != 0 {
		t.Fatalf("missing file must be zero value, got %+v, %v", f, err)
	}
}

func TestLoadMalformedYAMLNamesTheFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("profiles: [\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), path) {
		t.Fatalf("malformed YAML must fail and name its file: %v", err)
	}
}

// TestIsLoopbackURL pins the semantics against internal/api's isLoopbackHost,
// the other copy of this test. A case that changes here without changing there
// means MASTER_KEY and the plaintext-HTTP gate have started disagreeing about
// what counts as this machine.
func TestIsLoopbackURL(t *testing.T) {
	loopback := []string{
		"http://localhost:8080", "http://LOCALHOST", "http://localhost.:8080",
		"http://app.localhost:3000", "http://127.0.0.1:8080", "http://127.9.9.9",
		"http://[::1]:8080", "https://localhost",
	}
	remote := []string{
		"https://gw.example.com", "http://localhost.evil.com", "http://notlocalhost",
		"http://0.0.0.0:8080", "http://10.0.0.1", "http://[fe80::1]", "",
		// A scheme-less string parses as scheme "localhost" with an empty
		// host, so it must not be read as the loopback name it resembles.
		"localhost:8080",
		"://not a url",
	}
	for _, u := range loopback {
		if !isLoopbackURL(u) {
			t.Errorf("%q is this machine; MASTER_KEY must be usable", u)
		}
	}
	for _, u := range remote {
		if isLoopbackURL(u) {
			t.Errorf("%q is not this machine; MASTER_KEY must not be sent there", u)
		}
	}
}
