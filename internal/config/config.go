// Package config resolves where ferro connects and with which credential.
// Precedence — path: --config > FERRO_CONFIG > DefaultPath()
//
//	URL:  --gateway-url > FERRO_URL > profile.url > http://localhost:8080
//	key:  FERRO_API_KEY > profile.api_key_env deref > MASTER_KEY (loopback URL only) > ""
//
// The MASTER_KEY step is restricted because, unlike the two above it, nobody
// named it for ferro: it is the gateway server's own variable and is simply
// present in the shell on a gateway host. Honouring it for any URL would let
// `ferro --gateway-url https://elsewhere status`, typed in that shell, hand the
// gateway's root credential to a stranger.
//
// ferro stores no gateway data; this file holds connection profiles only.
package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// DefaultURL is where ferro looks when nothing else says otherwise.
const DefaultURL = "http://localhost:8080"

// The environment variables Resolve reads, and the KeySource label it reports
// for each source. Building the labels from the variable names is what keeps
// the two spellings from drifting: a doctor screen naming a variable nothing
// reads is worse than no label at all.
const (
	envURL       = "FERRO_URL"
	envFerroKey  = "FERRO_API_KEY"
	envMasterKey = "MASTER_KEY"

	keySourceFerroEnv  = "env:" + envFerroKey
	keySourceMasterEnv = "env:" + envMasterKey
	keySourceProfile   = "profile:"

	// keySourceMasterSkipped is reported instead of a credential when
	// MASTER_KEY is set but the gateway is remote. It rides on KeySource
	// rather than a new field so any display that already shows where the
	// credential came from also explains why there is none.
	keySourceMasterSkipped = keySourceMasterEnv + " skipped (gateway is not loopback)"
)

// Profile is one named connection. It holds no credential: api_key_env names
// an environment variable, so the config file never carries a secret.
type Profile struct {
	Name      string `yaml:"name"`
	URL       string `yaml:"url"`
	APIKeyEnv string `yaml:"api_key_env,omitempty"`
}

// File is the on-disk ferro config: connection profiles and nothing else.
type File struct {
	CurrentProfile string    `yaml:"current_profile,omitempty"`
	Profiles       []Profile `yaml:"profiles,omitempty"`
}

// Resolved is what the rest of ferro consumes. KeySource is for doctor-style
// display ("env:FERRO_API_KEY", "profile:PROD_KEY", "env:MASTER_KEY"), and is
// the only place a refused MASTER_KEY is visible -- APIKey is then empty and
// the gateway's 401 is all ferro would otherwise have to go on.
type Resolved struct {
	ProfileName string
	URL         string
	APIKey      string
	KeySource   string
}

// EnvConfigPath is the environment variable that overrides DefaultPath.
// --config on the CLI outranks it in turn (see the precedence note above). A
// config path names a filesystem location, never a credential, so unlike
// FERRO_API_KEY it carries nothing that must not appear in a shell history.
const EnvConfigPath = "FERRO_CONFIG"

// DefaultPath is the config location, empty when the OS reports no config dir.
func DefaultPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "ferro", "config.yaml")
}

// Load reads the config file. A missing file is not an error -- ferro works
// with flags and environment alone.
func Load(path string) (File, error) {
	var f File
	if path == "" {
		return f, nil
	}
	// Normalise before use so the file that is read and the file a parse error
	// names are spelled the same way, whatever lexical noise (`a/../b`, a
	// trailing slash) the flag or FERRO_CONFIG carried.
	clean := filepath.Clean(path)
	b, err := os.ReadFile(clean)
	if os.IsNotExist(err) {
		return f, nil
	}
	if err != nil {
		return f, err
	}
	if err := yaml.Unmarshal(b, &f); err != nil {
		return f, fmt.Errorf("parse %s: %w", clean, err)
	}
	return f, nil
}

// Resolve applies the precedence documented on this package to produce the
// connection ferro will use.
func Resolve(f File, flagURL, flagProfile string, getenv func(string) string) (Resolved, error) {
	r := Resolved{URL: DefaultURL}

	name := flagProfile
	if name == "" {
		name = f.CurrentProfile
	}
	var prof *Profile
	if name != "" {
		for i := range f.Profiles {
			if f.Profiles[i].Name == name {
				prof = &f.Profiles[i]
				break
			}
		}
		if prof == nil {
			names := make([]string, 0, len(f.Profiles))
			for _, p := range f.Profiles {
				names = append(names, p.Name)
			}
			sort.Strings(names)
			if len(names) == 0 {
				return r, fmt.Errorf("unknown profile %q (no profiles are defined)", name)
			}
			return r, fmt.Errorf("unknown profile %q (valid: %s)", name, strings.Join(names, ", "))
		}
	}
	if prof != nil {
		r.ProfileName = prof.Name
		if prof.URL != "" {
			r.URL = prof.URL
		}
	}
	if v := getenv(envURL); v != "" {
		r.URL = v
	}
	if flagURL != "" {
		r.URL = flagURL
	}

	switch {
	case getenv(envFerroKey) != "":
		r.APIKey, r.KeySource = getenv(envFerroKey), keySourceFerroEnv
	case prof != nil && prof.APIKeyEnv != "" && getenv(prof.APIKeyEnv) != "":
		r.APIKey, r.KeySource = getenv(prof.APIKeyEnv), keySourceProfile+prof.APIKeyEnv
	case getenv(envMasterKey) != "":
		// Loopback only -- see the package comment. Falling through to no
		// credential rather than erroring keeps the failure the gateway's to
		// report: it answers 401 and `ferro status` prints auth: unauthorized,
		// which is the same story an expired key tells.
		if isLoopbackURL(r.URL) {
			r.APIKey, r.KeySource = getenv(envMasterKey), keySourceMasterEnv
		} else {
			r.KeySource = keySourceMasterSkipped
		}
	}
	return r, nil
}

// isLoopbackURL reports whether raw names a host on this machine.
//
// internal/api holds the other copy of this test (isLoopbackHost, which gates
// plaintext HTTP). They are duplicated rather than shared because config sits
// below the client and must not import it to answer a question about a string,
// and exporting one of them would put an internal predicate in a package API
// for a single caller. Keep the two in step: if they disagree about what
// loopback means, ferro would ship MASTER_KEY to a host it also refuses to
// speak plaintext to, or withhold it from one it will.
func isLoopbackURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	// A trailing dot is the same name in DNS ("localhost." resolves like
	// "localhost"), so it must not be a way past this check.
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
