package cliparser_test

import (
	"testing"

	"github.com/shibukawa/tinybind-go/cliparser"
)

func TestDefaultLongNameAndConfigKey(t *testing.T) {
	if got := cliparser.DefaultLongName("webserver", "port"); got != "webserver-port" {
		t.Fatalf("DefaultLongName=%q", got)
	}
	if got := cliparser.ConfigKeyPath("webserver", "port"); got != "webserver.port" {
		t.Fatalf("ConfigKeyPath=%q", got)
	}
	if got := cliparser.DefaultLongName("webserver", "tls.enabled"); got != "webserver-tls-enabled" {
		t.Fatalf("nested DefaultLongName=%q", got)
	}
}

func TestParseDefaultPrefixedFlag(t *testing.T) {
	defs, err := cliparser.BuildDefs([]cliparser.FieldMeta{
		{Prefix: "webserver", Key: "port", Help: "port"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(defs) != 1 || defs[0].Longs[0] != "webserver-port" || defs[0].UsesOptOverride {
		t.Fatalf("defs=%+v", defs)
	}

	res, err := cliparser.Parse([]string{"--webserver-port", "8080"}, defs)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if res.Values["webserver.port"] != "8080" {
		t.Fatalf("Values=%v", res.Values)
	}
	if _, ok := res.Values["webserver-port"]; ok {
		t.Fatalf("map must use config key, not flag name: %v", res.Values)
	}
}

func TestParseOptLongAndShortSuppressDefault(t *testing.T) {
	defs, err := cliparser.BuildDefs([]cliparser.FieldMeta{
		{Prefix: "webserver", Key: "port", Opt: "port,p"},
	})
	if err != nil {
		t.Fatal(err)
	}
	d := defs[0]
	if !d.UsesOptOverride || d.ConfigKey != "webserver.port" {
		t.Fatalf("def=%+v", d)
	}
	if len(d.Longs) != 1 || d.Longs[0] != "port" {
		t.Fatalf("longs=%v", d.Longs)
	}
	if len(d.Shorts) != 1 || d.Shorts[0] != "p" {
		t.Fatalf("shorts=%v", d.Shorts)
	}

	// default prefixed name must not work
	if _, err := cliparser.Parse([]string{"--webserver-port", "1"}, defs); err == nil {
		t.Fatal("expected error for suppressed --webserver-port")
	}

	res, err := cliparser.Parse([]string{"--port", "80"}, defs)
	if err != nil {
		t.Fatal(err)
	}
	if res.Values["webserver.port"] != "80" {
		t.Fatalf("long Values=%v", res.Values)
	}

	res, err = cliparser.Parse([]string{"-p", "90"}, defs)
	if err != nil {
		t.Fatal(err)
	}
	if res.Values["webserver.port"] != "90" {
		t.Fatalf("short Values=%v", res.Values)
	}

	res, err = cliparser.Parse([]string{"-p9090"}, defs)
	if err != nil {
		t.Fatal(err)
	}
	if res.Values["webserver.port"] != "9090" {
		t.Fatalf("attached short Values=%v", res.Values)
	}
}

func TestParseMissingFlagsAbsent(t *testing.T) {
	defs, err := cliparser.BuildDefs([]cliparser.FieldMeta{
		{Prefix: "webserver", Key: "port"},
		{Prefix: "webserver", Key: "host"},
	})
	if err != nil {
		t.Fatal(err)
	}
	res, err := cliparser.Parse([]string{"--webserver-port", "1"}, defs)
	if err != nil {
		t.Fatal(err)
	}
	if res.Values["webserver.port"] != "1" {
		t.Fatalf("Values=%v", res.Values)
	}
	if _, ok := res.Values["webserver.host"]; ok {
		t.Fatalf("unset flag must be absent: %v", res.Values)
	}
}

func TestParseEqualsAndBoolAndArray(t *testing.T) {
	defs, err := cliparser.BuildDefs([]cliparser.FieldMeta{
		{Prefix: "webserver", Key: "port"},
		{Prefix: "webserver", Key: "tls.enabled", Kind: cliparser.KindBool},
		{Prefix: "webserver", Key: "cors_origins", Kind: cliparser.KindArray},
	})
	if err != nil {
		t.Fatal(err)
	}
	res, err := cliparser.Parse([]string{
		"--webserver-port=8080",
		"--webserver-tls-enabled",
		"--webserver-cors_origins", "a.example",
		"--webserver-cors_origins", "b.example",
		"positional",
	}, defs)
	if err != nil {
		t.Fatal(err)
	}
	if res.Values["webserver.port"] != "8080" {
		t.Fatalf("port=%v", res.Values)
	}
	if res.Values["webserver.tls.enabled"] != "true" {
		t.Fatalf("tls=%v", res.Values)
	}
	if got := res.Multi["webserver.cors_origins"]; len(got) != 2 || got[0] != "a.example" || got[1] != "b.example" {
		t.Fatalf("multi=%v", res.Multi)
	}
	if len(res.Rest) != 1 || res.Rest[0] != "positional" {
		t.Fatalf("Rest=%v", res.Rest)
	}
}

func TestBuildDefsThenParseFixture(t *testing.T) {
	// Shipped helper path used by codegen: FieldMeta → BuildDefs → Parse.
	fields := []cliparser.FieldMeta{
		{Prefix: "webserver", Key: "port", Opt: "port,p", Help: "HTTP listen port"},
		{Prefix: "webserver", Key: "read_timeout", Help: "read timeout"},
	}
	defs, err := cliparser.BuildDefs(fields)
	if err != nil {
		t.Fatal(err)
	}
	res, err := cliparser.Parse([]string{"-p", "443", "--webserver-read_timeout", "5s"}, defs)
	if err != nil {
		t.Fatal(err)
	}
	if res.Values["webserver.port"] != "443" {
		t.Fatalf("port=%v", res.Values)
	}
	if res.Values["webserver.read_timeout"] != "5s" {
		t.Fatalf("read_timeout=%v", res.Values)
	}
}
