package registry

import (
	"os"
	"path/filepath"
	"testing"
)

func writeRegistryFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	err := os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLookupValid(t *testing.T) {
	dir := t.TempDir()
	path := writeRegistryFile(t, dir, "images.toml", `
version = 1

[images.ubuntu-noble]
url = "https://example.com/ubuntu-noble.qcow2"

[images.debian-12]
url = "https://example.com/debian-12.qcow2"
`)

	reg := New(path)

	url, err := reg.Lookup("ubuntu-noble")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "https://example.com/ubuntu-noble.qcow2" {
		t.Fatalf("unexpected url: %s", url)
	}
}

func TestLookupNotFound(t *testing.T) {
	dir := t.TempDir()
	path := writeRegistryFile(t, dir, "images.toml", `
version = 1

[images.ubuntu-noble]
url = "https://example.com/ubuntu-noble.qcow2"
`)

	reg := New(path)

	_, err := reg.Lookup("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent image")
	}
}

func TestVersionMissing(t *testing.T) {
	dir := t.TempDir()
	path := writeRegistryFile(t, dir, "images.toml", `
[images.ubuntu-noble]
url = "https://example.com/ubuntu-noble.qcow2"
`)

	reg := New(path)

	_, err := reg.Lookup("ubuntu-noble")
	if err == nil {
		t.Fatal("expected error for missing version")
	}
}

func TestVersionUnsupported(t *testing.T) {
	dir := t.TempDir()
	path := writeRegistryFile(t, dir, "images.toml", `
version = 99

[images.ubuntu-noble]
url = "https://example.com/ubuntu-noble.qcow2"
`)

	reg := New(path)

	_, err := reg.Lookup("ubuntu-noble")
	if err == nil {
		t.Fatal("expected error for unsupported version")
	}
}

func TestMultipleFiles(t *testing.T) {
	dir := t.TempDir()
	path1 := writeRegistryFile(t, dir, "shipped.toml", `
version = 1

[images.ubuntu-noble]
url = "https://example.com/old.qcow2"
`)
	path2 := writeRegistryFile(t, dir, "user.toml", `
version = 1

[images.ubuntu-noble]
url = "https://example.com/new.qcow2"
`)

	reg := New(path1, path2)

	url, err := reg.Lookup("ubuntu-noble")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "https://example.com/new.qcow2" {
		t.Fatalf("expected user file to override shipped, got: %s", url)
	}
}

func TestNonexistentFileIgnored(t *testing.T) {
	dir := t.TempDir()
	path := writeRegistryFile(t, dir, "images.toml", `
version = 1

[images.ubuntu-noble]
url = "https://example.com/ubuntu-noble.qcow2"
`)

	reg := New("/nonexistent/path/images.toml", path)

	url, err := reg.Lookup("ubuntu-noble")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "https://example.com/ubuntu-noble.qcow2" {
		t.Fatalf("unexpected url: %s", url)
	}
}
