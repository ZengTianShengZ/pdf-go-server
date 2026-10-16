package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInjectReportData_ReplacesObjectOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "t.html")
	src := `<script>window.__REPORT_DATA__ = {"name":"old"};</script>`
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := InjectReportData(path, []byte(`{"name":"Jane","city":"SF"}`))
	if err != nil {
		t.Fatal(err)
	}

	s := string(out)
	if !strings.Contains(s, `window.__REPORT_DATA__ = `) {
		t.Fatalf("assignment missing: %s", s)
	}
	if !strings.Contains(s, `"name":"Jane"`) || !strings.Contains(s, `"city":"SF"`) {
		t.Fatalf("new data missing: %s", s)
	}
	if strings.Contains(s, `"name":"old"`) {
		t.Fatalf("old data still present: %s", s)
	}
}

func TestInjectReportData_MultilineObject(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "t.html")
	src := `<script>
        window.__REPORT_DATA__ = {
            "name": "John Doe",
            "city": "Anytown"
        };
    </script>`
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := InjectReportData(path, []byte(`{"name":"Ada"}`))
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, `"name":"Ada"`) {
		t.Fatalf("inject failed: %s", s)
	}
	if strings.Contains(s, "John Doe") {
		t.Fatalf("old multiline data remains: %s", s)
	}
}

func TestInjectReportData_NestedObject(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "t.html")
	src := `window.__REPORT_DATA__ = {"a":{"b":1}};`
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := InjectReportData(path, []byte(`{"a":{"b":2,"c":3}}`))
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if s != `window.__REPORT_DATA__ = {"a":{"b":2,"c":3}};` {
		t.Fatalf("got %s", s)
	}
}

func TestInjectReportData_MissingMarker(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "t.html")
	if err := os.WriteFile(path, []byte(`<html></html>`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := InjectReportData(path, []byte(`{}`)); err == nil {
		t.Fatal("expected error")
	}
}

func TestEnsureJSONObjectRoot(t *testing.T) {
	if err := EnsureJSONObjectRoot([]byte(`{"a":1}`)); err != nil {
		t.Fatal(err)
	}
	if err := EnsureJSONObjectRoot([]byte(`[]`)); err == nil {
		t.Fatal("expected error for array")
	}
	if err := EnsureJSONObjectRoot([]byte(`"x"`)); err == nil {
		t.Fatal("expected error for string")
	}
}
