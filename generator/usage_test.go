package generator_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/httpbind-go/generator"
)

func TestEmit_DecodeOnlyHasNoNetHTTP(t *testing.T) {
	dir := t.TempDir()
	writeTempModule(t, dir)
	src := `package sample
import "github.com/shibukawa/httpbind-go"
type Note struct { Text string ` + "`json:\"text\"`" + ` }
type Unused struct { ID int }
func use() { _, _ = httpbinder.DecodeJSON[Note](nil) }
`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	tidyTempModule(t, dir)
	plan, err := generator.New(generator.DefaultOptions()).Analyze(dir)
	if err != nil {
		t.Fatal(err)
	}
	code, err := generator.Emit(plan)
	if err != nil {
		t.Fatal(err)
	}
	s := string(code)
	if strings.Contains(s, `"net/http"`) || strings.Contains(s, "RegisterBind") || strings.Contains(s, "RegisterWrite") {
		t.Fatalf("decode-only output contains HTTP mapping:\n%s", s)
	}
	if !strings.Contains(s, "RegisterDecode[Note]") {
		t.Fatalf("missing decoder:\n%s", s)
	}
	if strings.Contains(s, "Unused") {
		t.Fatalf("unused model emitted:\n%s", s)
	}
}

func TestEmit_EncodeOnlyHasNoNetHTTP(t *testing.T) {
	dir := t.TempDir()
	writeTempModule(t, dir)
	src := `package sample
import (
 "io"
 "github.com/shibukawa/httpbind-go"
)
type Note struct { Text string ` + "`json:\"text\"`" + ` }
type Unused struct { ID int }
func use() { _ = httpbinder.EncodeJSON[Note](io.Discard, Note{}) }
`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	tidyTempModule(t, dir)
	plan, err := generator.New(generator.DefaultOptions()).Analyze(dir)
	if err != nil {
		t.Fatal(err)
	}
	code, err := generator.Emit(plan)
	if err != nil {
		t.Fatal(err)
	}
	s := string(code)
	if strings.Contains(s, `"net/http"`) || strings.Contains(s, "RegisterBind") || strings.Contains(s, "RegisterWrite") || strings.Contains(s, "RegisterDecode") {
		t.Fatalf("encode-only output contains unrelated mapping:\n%s", s)
	}
	if !strings.Contains(s, "RegisterEncode[Note]") {
		t.Fatalf("missing encoder:\n%s", s)
	}
	if strings.Contains(s, "Unused") {
		t.Fatalf("unused model emitted:\n%s", s)
	}
}

func TestEmit_ScanRowsOnlyBuildsReflectionFreeScanner(t *testing.T) {
	dir := t.TempDir()
	writeTempModule(t, dir)
	src := `package sample
import (
 "database/sql"
 "github.com/shibukawa/httpbind-go"
)
type Role struct { ID int ` + "`db:\"role_id\" groupkey:\"\"`" + `; Name string ` + "`db:\"role_name\"`" + ` }
type User struct { ID int ` + "`db:\"user_id\" groupkey:\"\"`" + `; Name string ` + "`db:\"user_name\"`" + `; Roles []Role }
type Organization struct { ID int ` + "`db:\"org_id\" groupkey:\"\"`" + `; Name string ` + "`db:\"org_name\"`" + `; Users []User }
func use(rows *sql.Rows) { _, _ = httpbinder.ScanRows[Organization](rows) }
`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	tidyTempModule(t, dir)
	plan, err := generator.New(generator.DefaultOptions()).Analyze(dir)
	if err != nil {
		t.Fatal(err)
	}
	code, err := generator.Emit(plan)
	if err != nil {
		t.Fatal(err)
	}
	s := string(code)
	for _, want := range []string{"RegisterScanRows[Organization]", "func scanOrganizationRows", "sqlmap.ForEach", "Users = append"} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q:\n%s", want, s)
		}
	}
	if strings.Contains(s, `"net/http"`) || strings.Contains(s, `"reflect"`) {
		t.Fatalf("SQL-only output has unrelated dependency:\n%s", s)
	}
	if err := os.WriteFile(filepath.Join(dir, "httpbinder_gen.go"), code, 0o644); err != nil {
		t.Fatal(err)
	}
	runtimeTest := `package sample
import (
 "context"
 "database/sql"
 "database/sql/driver"
 "io"
 "testing"
 "github.com/shibukawa/httpbind-go"
)
type fixtureDriver struct{}
type fixtureConn struct{}
type fixtureRows struct{ pos int }
func (fixtureDriver) Open(string)(driver.Conn,error){return fixtureConn{},nil}
func (fixtureConn) Prepare(string)(driver.Stmt,error){return nil,driver.ErrSkip}
func (fixtureConn) Close()error{return nil}
func (fixtureConn) Begin()(driver.Tx,error){return nil,driver.ErrSkip}
func (fixtureConn) QueryContext(context.Context,string,[]driver.NamedValue)(driver.Rows,error){return &fixtureRows{},nil}
func (*fixtureRows) Columns()[]string{return []string{"org_id","org_name","user_id","user_name","role_id","role_name"}}
func (*fixtureRows) Close()error{return nil}
func (r *fixtureRows) Next(dest []driver.Value)error{
 data:=[][]driver.Value{{int64(1),"Acme",int64(10),"A",int64(100),"Admin"},{int64(1),"Acme",int64(10),"A",int64(101),"Editor"},{int64(1),"Acme",int64(11),"B",nil,nil},{int64(1),"Acme",int64(10),"A",int64(100),"Admin"},{int64(2),"Empty",nil,nil,nil,nil}}
 if r.pos>=len(data){return io.EOF};copy(dest,data[r.pos]);r.pos++;return nil
}
func init(){sql.Register("httpbinder_fixture",fixtureDriver{})}
func TestGeneratedTree(t *testing.T){
 db,err:=sql.Open("httpbinder_fixture","");if err!=nil{t.Fatal(err)};defer db.Close()
 rows,err:=db.QueryContext(context.Background(),"select");if err!=nil{t.Fatal(err)};defer rows.Close()
 got,err:=httpbinder.ScanRows[Organization](rows);if err!=nil{t.Fatal(err)}
 if len(got)!=2||got[0].Name!="Acme"||len(got[0].Users)!=2||len(got[0].Users[0].Roles)!=2||got[0].Users[0].Roles[1].Name!="Editor"||got[0].Users[1].Name!="B"||len(got[0].Users[1].Roles)!=0||len(got[1].Users)!=0{t.Fatalf("tree: %+v",got)}
}
`
	if err := os.WriteFile(filepath.Join(dir, "runtime_test.go"), []byte(runtimeTest), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOCACHE=/tmp/httpbind-go-cache")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated SQL build: %v\n%s", err, out)
	}
}

func TestAnalyze_CustomDiscoverySymbol(t *testing.T) {
	dir := t.TempDir()
	writeTempModule(t, dir)
	if err := os.Mkdir(filepath.Join(dir, "compat"), 0o755); err != nil {
		t.Fatal(err)
	}
	compat := `package compat
import (
 "io"
 hb "github.com/shibukawa/httpbind-go"
)
type File = hb.File
func DecodeJSON[T any](io.Reader) (T,error) { var z T; return z,nil }
`
	main := `package sample
import "tempmod/compat"
type Note struct { Text string; Attachment compat.File ` + "`payload:\"attachment\"`" + ` }
func use() { _, _ = compat.DecodeJSON[Note](nil) }
`
	if err := os.WriteFile(filepath.Join(dir, "compat", "compat.go"), []byte(compat), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(main), 0o644); err != nil {
		t.Fatal(err)
	}
	tidyTempModule(t, dir)
	g := generator.New(generator.Options{
		DecodeJSON: generator.PatternSet[generator.SymbolPattern]{Set: []generator.SymbolPattern{{PackagePath: "tempmod/compat", Name: "DecodeJSON"}}},
		FileTypes:  generator.PatternSet[generator.TypePattern]{Set: []generator.TypePattern{{PackagePath: "tempmod/compat", Name: "File"}}},
	})
	plan, err := g.Analyze(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Types) != 1 || plan.Types[0].DirectUsage != generator.UsageDecodeJSON {
		t.Fatalf("usage: %+v", plan.Types)
	}
	if len(plan.Types[0].Fields) != 2 || plan.Types[0].Fields[1].Kind != "file" {
		t.Fatalf("custom File type: %+v", plan.Types[0].Fields)
	}
}

func TestAnalyze_DiscoversInferredWriteType(t *testing.T) {
	dir := t.TempDir()
	writeTempModule(t, dir)
	src := `package sample
import (
 "net/http"
 "github.com/shibukawa/httpbind-go"
)
type Response struct{ ID int }
func use(w http.ResponseWriter,r *http.Request){ _ = httpbinder.Write(w,r,Response{ID:1}) }
`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	tidyTempModule(t, dir)
	plan, err := generator.New(generator.DefaultOptions()).Analyze(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Types) != 1 || plan.Types[0].DirectUsage != generator.UsageWrite {
		t.Fatalf("usage: %+v", plan.Types)
	}
}
