package generator_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
)

const unformattedSQL = "export statement FindUser(id: int): sql.one<UserRow> {SELECT id, name FROM users WHERE id = {id}}"

func formatSet(t *testing.T) generator.CommandSet {
	t.Helper()
	set, err := generator.NewCommandSet(generator.FormatCommand(generator.DefaultOptions()))
	if err != nil {
		t.Fatal(err)
	}
	return set
}

func runFmt(t *testing.T, args []string, stdin string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := formatSet(t).Run(context.Background(), append([]string{"fmt"}, args...), generator.CommandIO{
		Stdin:  strings.NewReader(stdin),
		Stdout: &stdout,
		Stderr: &stderr,
	})
	return code, stdout.String(), stderr.String()
}

func TestFormatCommandListsUnformattedFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "users.tb.sql")
	if err := os.WriteFile(path, []byte(unformattedSQL), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := runFmt(t, []string{"-l", "-dir", dir}, "")
	if code != 1 {
		t.Fatalf("exit %d, stderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "users.tb.sql") {
		t.Fatalf("path not listed: %q", stdout)
	}
	if after, _ := os.ReadFile(path); string(after) != unformattedSQL {
		t.Fatalf("-l wrote to the file:\n%s", after)
	}
}

func TestFormatCommandWritesAndThenReportsClean(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "users.tb.sql")
	if err := os.WriteFile(path, []byte(unformattedSQL), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := runFmt(t, []string{"-w", "-dir", dir}, ""); code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr)
	}
	formatted, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(formatted), "\n  FROM users\n") {
		t.Fatalf("not laid out:\n%s", formatted)
	}
	// A second run has nothing to do, which is what makes -l usable in CI.
	code, stdout, _ := runFmt(t, []string{"-l", "-dir", dir}, "")
	if code != 0 || stdout != "" {
		t.Fatalf("second run reported %d %q", code, stdout)
	}
}

func TestFormatCommandLeavesBrokenSourceUntouched(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "broken.tb.sql")
	broken := "export statement Oops(: sql.exec {SELECT 1}"
	if err := os.WriteFile(path, []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := runFmt(t, []string{"-w", "-dir", dir}, "")
	if code != 1 {
		t.Fatalf("exit %d", code)
	}
	if stderr == "" {
		t.Fatal("no diagnostic reported")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != broken {
		t.Fatalf("the file was rewritten:\n%s", after)
	}
}

func TestFormatCommandFiltersStdin(t *testing.T) {
	code, stdout, stderr := runFmt(t, []string{"-as", "sql"}, unformattedSQL)
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "\n  WHERE id = {id}\n") {
		t.Fatalf("stdin was not formatted:\n%s", stdout)
	}
}
