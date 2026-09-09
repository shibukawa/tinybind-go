package configbind_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/cliparser"
	"github.com/shibukawa/tinybind-go/configbind"
)

// registerSecretProbe binds a struct whose key names match no sensitive token,
// so any masking observed comes from the origin alone.
type secretProbe struct {
	Hook string
	Note string
}

func registerSecretProbe(t *testing.T, secrets map[string]string) *secretProbe {
	t.Helper()
	configbind.ResetTargets()
	configbind.ResetDefinitions()
	configbind.Register[secretProbe](configbind.Definition{
		TypeName:  "secretProbe",
		Prefix:    "webhook",
		KnownKeys: []string{"webhook.hook", "webhook.note"},
		FlagMetas: []cliparser.FieldMeta{
			{Prefix: "webhook", Key: "hook"},
			{Prefix: "webhook", Key: "note"},
		},
		Secrets: secrets,
		Apply: func(dst any, o *configbind.Overlay) error {
			p := dst.(*secretProbe)
			p.Hook, _ = o.GetString("webhook.hook")
			p.Note, _ = o.GetString("webhook.note")
			return nil
		},
	})
	return configbind.Bind[secretProbe]("webhook")
}

func provenanceOf(t *testing.T, result *configbind.LoadResult, key string) configbind.ProvenanceEntry {
	t.Helper()
	for _, e := range result.Provenance() {
		if e.Key == key {
			return e
		}
	}
	t.Fatalf("%s is not in provenance", key)
	return configbind.ProvenanceEntry{}
}

func baseOptions(t *testing.T) configbind.LoadOptions {
	t.Helper()
	return configbind.LoadOptions{
		Vendor:  "acme-missing-vendor-xyz",
		Tool:    "tool-missing-xyz",
		Args:    []string{},
		Environ: []string{},
	}
}

func TestEnvSecretFilesMaskByOriginAndKeepTheStructValue(t *testing.T) {
	cfg := registerSecretProbe(t, nil)
	dir := t.TempDir()
	local := writeEnvFile(t, dir, ".env.local", "WEBHOOK_HOOK=https://h/abc123\n")
	opts := baseOptions(t)
	opts.EnvFiles = []configbind.EnvFile{{Path: local, Secret: true}}
	result, err := configbind.Load(opts)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Hook != "https://h/abc123" {
		t.Fatalf("struct value=%q must stay unmasked", cfg.Hook)
	}
	e := provenanceOf(t, result, "webhook.hook")
	if !e.Masked || e.Value == "https://h/abc123" {
		t.Fatalf("entry=%+v want masked", e)
	}
	if e.Place != configbind.PlaceEnvFile+configbind.Place(local) {
		t.Fatalf("place=%q", e.Place)
	}
	if len(result.EnvFiles) != 1 || result.EnvFiles[0] != (configbind.EnvFile{Path: local, Secret: true}) {
		t.Fatalf("EnvFiles=%v", result.EnvFiles)
	}
}

func TestEnvSecretDirsReadEntriesAsVariables(t *testing.T) {
	cfg := registerSecretProbe(t, nil)
	secrets := filepath.Join(t.TempDir(), "run", "secrets")
	// Kubernetes layout: ..data holds the files, keys are symlinks into it.
	data := filepath.Join(secrets, "..data")
	if err := os.MkdirAll(data, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(data, "WEBHOOK_HOOK"), []byte("s3cret\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("..data", "WEBHOOK_HOOK"), filepath.Join(secrets, "WEBHOOK_HOOK")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(secrets, ".hidden"), []byte("WEBHOOK_NOTE=x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(secrets, "WEBHOOK_NOTE"), 0o755); err != nil {
		t.Fatal(err)
	}
	opts := baseOptions(t)
	opts.EnvSecretDirs = []string{secrets, filepath.Join(secrets, "missing")}
	result, err := configbind.Load(opts)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Hook != "s3cret" {
		t.Fatalf("Hook=%q want trailing line endings stripped", cfg.Hook)
	}
	if cfg.Note != "" {
		t.Fatalf("Note=%q; a directory entry must be skipped", cfg.Note)
	}
	e := provenanceOf(t, result, "webhook.hook")
	if !e.Masked {
		t.Fatalf("entry=%+v want masked by origin", e)
	}
	file, ok := configbind.EnvFileOf(e.Place)
	if !ok || file != filepath.Join(secrets, "WEBHOOK_HOOK") {
		t.Fatalf("EnvFileOf(%q)=%q,%v", e.Place, file, ok)
	}
	if len(result.EnvSecretDirs) != 1 || result.EnvSecretDirs[0] != secrets {
		t.Fatalf("EnvSecretDirs=%v want only the present directory", result.EnvSecretDirs)
	}
}

func TestEnvSecretDirsUnreadableIsAnError(t *testing.T) {
	registerSecretProbe(t, nil)
	file := writeEnvFile(t, t.TempDir(), "not-a-dir", "")
	opts := baseOptions(t)
	opts.EnvSecretDirs = []string{file}
	_, err := configbind.Load(opts)
	if err == nil || !strings.Contains(err.Error(), "read env dir") {
		t.Fatalf("err=%v want a directory read error", err)
	}
}

func TestEnvFilesReadInGivenOrderWithSecretsInterleaved(t *testing.T) {
	// dotenv-flow order: .env < .env.local < .env.stg < .env.stg.local, so a
	// plain per-environment file outranks the shared local secrets.
	cfg := registerSecretProbe(t, nil)
	dir := t.TempDir()
	base := writeEnvFile(t, dir, ".env", "WEBHOOK_HOOK=base\nWEBHOOK_NOTE=base\n")
	local := writeEnvFile(t, dir, ".env.local", "WEBHOOK_HOOK=local\nWEBHOOK_NOTE=local\n")
	stg := writeEnvFile(t, dir, ".env.stg", "WEBHOOK_NOTE=stg\n")
	stgLocal := writeEnvFile(t, dir, ".env.stg.local", "WEBHOOK_HOOK=stg-local\n")
	opts := baseOptions(t)
	opts.EnvFiles = []configbind.EnvFile{
		{Path: base},
		{Path: local, Secret: true},
		{Path: stg},
		{Path: stgLocal, Secret: true},
	}
	result, err := configbind.Load(opts)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Hook != "stg-local" || cfg.Note != "stg" {
		t.Fatalf("cfg=%+v want slice order, not plain-then-secret", cfg)
	}
	hook := provenanceOf(t, result, "webhook.hook")
	if !hook.Masked || hook.Place != configbind.PlaceEnvFile+configbind.Place(stgLocal) {
		t.Fatalf("hook=%+v want masked from .env.stg.local", hook)
	}
	note := provenanceOf(t, result, "webhook.note")
	if note.Masked || note.Value != "stg" || note.Place != configbind.PlaceEnvFile+configbind.Place(stg) {
		t.Fatalf("note=%+v; a plain file above a secret one is not secret", note)
	}
	if len(result.EnvFiles) != 4 || !result.EnvFiles[1].Secret || result.EnvFiles[2].Secret {
		t.Fatalf("EnvFiles=%v", result.EnvFiles)
	}
}

func TestEnvSecretDirsSitAboveFilesAndBelowProcess(t *testing.T) {
	cfg := registerSecretProbe(t, nil)
	dir := t.TempDir()
	local := writeEnvFile(t, dir, ".env.local", "WEBHOOK_HOOK=local\nWEBHOOK_NOTE=local\n")
	secrets := filepath.Join(dir, "secrets")
	if err := os.Mkdir(secrets, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"WEBHOOK_HOOK", "WEBHOOK_NOTE"} {
		if err := os.WriteFile(filepath.Join(secrets, name), []byte("mounted"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	opts := baseOptions(t)
	opts.EnvFiles = []configbind.EnvFile{{Path: local, Secret: true}}
	opts.EnvSecretDirs = []string{secrets}
	opts.Environ = []string{"WEBHOOK_NOTE=process"}
	result, err := configbind.Load(opts)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Hook != "mounted" || cfg.Note != "process" {
		t.Fatalf("cfg=%+v want dir over file, and process over all", cfg)
	}
	if hook := provenanceOf(t, result, "webhook.hook"); !hook.Masked {
		t.Fatalf("hook=%+v want masked", hook)
	}
	note := provenanceOf(t, result, "webhook.note")
	if note.Masked || note.Place != configbind.PlaceEnv || note.Value != "process" {
		t.Fatalf("note=%+v; a process value is shown even when a secret source also set the name", note)
	}
}

func TestEnvSecretOriginOutranksShowTagButNotHide(t *testing.T) {
	dir := t.TempDir()
	local := writeEnvFile(t, dir, ".env.local", "WEBHOOK_HOOK=shown?\nWEBHOOK_NOTE=hidden?\n")
	registerSecretProbe(t, map[string]string{"webhook.hook": "show", "webhook.note": "hide"})
	opts := baseOptions(t)
	opts.EnvFiles = []configbind.EnvFile{{Path: local, Secret: true}}
	result, err := configbind.Load(opts)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if e := provenanceOf(t, result, "webhook.hook"); !e.Masked {
		t.Fatalf("show tag must lose to origin: %+v", e)
	}
	for _, e := range result.Provenance() {
		if e.Key == "webhook.note" {
			t.Fatalf("hide must drop the entry: %+v", e)
		}
	}
}

func TestEnvSecretOriginTaintsTOMLInterpolation(t *testing.T) {
	cfg := registerSecretProbe(t, nil)
	dir := t.TempDir()
	plain := writeEnvFile(t, dir, ".env", "HOST=h.example\n")
	local := writeEnvFile(t, dir, ".env.local", "HOOK_TOKEN=abc123\n")
	tomlPath := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(tomlPath, []byte("[webhook]\nhook = \"https://${HOST}/${HOOK_TOKEN}\"\nnote = \"https://${HOST}/public\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	opts := baseOptions(t)
	opts.ExplicitConfigPath = tomlPath
	opts.EnvFiles = []configbind.EnvFile{{Path: plain}, {Path: local, Secret: true}}
	result, err := configbind.Load(opts)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Hook != "https://h.example/abc123" {
		t.Fatalf("Hook=%q", cfg.Hook)
	}
	hook := provenanceOf(t, result, "webhook.hook")
	if !hook.Masked || hook.Place != configbind.PlaceFile {
		t.Fatalf("hook=%+v want masked, still file_toml", hook)
	}
	note := provenanceOf(t, result, "webhook.note")
	if note.Masked || note.Value != "https://h.example/public" {
		t.Fatalf("note=%+v; plain-origin expansion keeps today's policy", note)
	}
}

func TestEnvSecretOriginClearsWhenCLIOverrides(t *testing.T) {
	registerSecretProbe(t, nil)
	local := writeEnvFile(t, t.TempDir(), ".env.local", "WEBHOOK_HOOK=from-secret\n")
	opts := baseOptions(t)
	opts.EnvFiles = []configbind.EnvFile{{Path: local, Secret: true}}
	opts.Args = []string{"--webhook-hook", "from-cli"}
	result, err := configbind.Load(opts)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	e := provenanceOf(t, result, "webhook.hook")
	if e.Masked || e.Place != configbind.PlaceCLI || e.Value != "from-cli" {
		t.Fatalf("entry=%+v; a CLI value has another origin", e)
	}
}

func TestEnvSecretEmptyValueIsSetAndUnmasked(t *testing.T) {
	cfg := registerSecretProbe(t, nil)
	secrets := t.TempDir()
	if err := os.WriteFile(filepath.Join(secrets, "WEBHOOK_HOOK"), []byte("\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	opts := baseOptions(t)
	opts.EnvSecretDirs = []string{secrets}
	result, err := configbind.Load(opts)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Hook != "" {
		t.Fatalf("Hook=%q", cfg.Hook)
	}
	if e := provenanceOf(t, result, "webhook.hook"); e.Masked || e.Value != "" {
		t.Fatalf("entry=%+v; nothing to mask", e)
	}
}
