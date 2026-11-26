package node

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func tarGz(name string, body []byte) []byte {
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	tw := tar.NewWriter(gz)
	_ = tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg})
	_, _ = tw.Write(body)
	_ = tw.Close()
	_ = gz.Close()
	return b.Bytes()
}

func zipOf(name string, body []byte) []byte {
	var b bytes.Buffer
	zw := zip.NewWriter(&b)
	w, _ := zw.Create(name)
	_, _ = w.Write(body)
	_ = zw.Close()
	return b.Bytes()
}

func TestAssetName(t *testing.T) {
	for _, c := range [][3]string{
		{"linux", "amd64", "nirad-" + NiradVersion + "-linux-amd64.tar.gz"},
		{"darwin", "arm64", "nirad-" + NiradVersion + "-darwin-arm64.tar.gz"},
		{"windows", "amd64", "nirad-" + NiradVersion + "-windows-amd64.zip"},
	} {
		if got, err := AssetName(c[0], c[1]); err != nil || got != c[2] {
			t.Errorf("%s/%s: %q %v", c[0], c[1], got, err)
		}
	}
	if _, err := AssetName("windows", "arm64"); err == nil {
		t.Error("unsupported platform accepted")
	}
}

func TestExtractAndChecksum(t *testing.T) {
	body := []byte("binary")
	for asset, archive := range map[string][]byte{"x.tar.gz": tarGz("nirad", body), "x.zip": zipOf("nirad.exe", body)} {
		name := "nirad"
		if strings.HasSuffix(asset, ".zip") {
			name = "nirad.exe"
		}
		got, err := extract(archive, asset, name)
		if err != nil || !bytes.Equal(got, body) {
			t.Fatalf("%s: %q %v", asset, got, err)
		}
	}
	sums := []byte("aa  other.tar.gz\n" + strings.Repeat("b", 64) + "  want.zip\n")
	if s, err := checksumFor(sums, "want.zip"); err != nil || s != strings.Repeat("b", 64) {
		t.Fatalf("checksumFor: %q %v", s, err)
	}
	if _, err := checksumFor(sums, "missing.zip"); err == nil {
		t.Fatal("missing asset must be an error")
	}
}

// Full install from a fake release server: verified archive installs and is
// recorded; a tampered archive is refused and nothing is recorded.
func TestInstallNirad(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a shell script as the fake binary")
	}
	asset, err := AssetName(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Skip(err)
	}
	script := []byte("#!/bin/sh\necho v0.1.1\n")
	archive := tarGz("nirad", script)
	sum := sha256.Sum256(archive)
	tamper := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch strings.TrimPrefix(r.URL.Path, "/") {
		case "SHA256SUMS":
			fmt.Fprintf(w, "%s  %s\n", hex.EncodeToString(sum[:]), asset)
		case asset:
			if tamper {
				_, _ = w.Write(tarGz("nirad", []byte("#!/bin/sh\necho evil\n")))
				return
			}
			_, _ = w.Write(archive)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	t.Setenv("NIRA_RELEASE_BASE", srv.URL)
	cfg := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("NIRA_NODE_CONFIG", cfg)

	data := t.TempDir()
	in, err := InstallNirad(context.Background(), data, func(string, int64, int64) {})
	if err != nil {
		t.Fatal(err)
	}
	if in.NiradPath != filepath.Join(data, "bin", "nirad") || in.NiradVersion != "v0.1.1" {
		t.Fatalf("install record %+v", in)
	}
	got, ok := LoadInstall()
	if !ok || got.DataDir != data || got.NiradPath != in.NiradPath {
		t.Fatalf("LoadInstall %+v %v", got, ok)
	}

	tamper = true
	_ = os.Remove(cfg)
	if _, err := InstallNirad(context.Background(), t.TempDir(), func(string, int64, int64) {}); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("tampered archive: %v", err)
	}
	if _, ok := LoadInstall(); ok {
		t.Fatal("a refused install must not be recorded")
	}
	if _, err := InstallNirad(context.Background(), "relative/dir", func(string, int64, int64) {}); err == nil {
		t.Fatal("relative data dir accepted")
	}
}
