package node

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// NiradVersion is the nirad release this app installs. It is pinned (not
// "latest") so an app version always runs the node version it was tested with.
const NiradVersion = "v0.1.1"

// ReleaseBase is where release archives and SHA256SUMS are downloaded from.
// NIRA_RELEASE_BASE overrides it (a mirror, or a local server in tests).
func ReleaseBase() string {
	if b := os.Getenv("NIRA_RELEASE_BASE"); b != "" {
		return strings.TrimRight(b, "/")
	}
	return "https://github.com/nnlgsakib/nira-chain/releases/download/" + NiradVersion
}

// Install is the app's bootstrap record, kept in the OS's standard per-user
// config folder (ConfigPath), so it is found again on every start:
//
//	Windows  %APPDATA%\NiraNode\config.json
//	macOS    ~/Library/Application Support/NiraNode/config.json
//	Linux    ~/.config/NiraNode/config.json   ($XDG_CONFIG_HOME respected)
type Install struct {
	DataDir      string    `json:"data_dir"`      // node homes, settings, bin/
	NiradPath    string    `json:"nirad_path"`    // installed binary
	NiradVersion string    `json:"nirad_version"` // release it came from
	InstalledAt  time.Time `json:"installed_at"`
}

// ConfigPath is the bootstrap file's location. NIRA_NODE_CONFIG overrides it.
func ConfigPath() (string, error) {
	if p := os.Getenv("NIRA_NODE_CONFIG"); p != "" {
		return p, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "NiraNode", "config.json"), nil
}

// DefaultDataDir is the suggested data folder (the user may pick another).
func DefaultDataDir() string {
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "windows":
		if d := os.Getenv("LOCALAPPDATA"); d != "" {
			return filepath.Join(d, "NiraNode")
		}
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "NiraNode")
	default:
		if d := os.Getenv("XDG_DATA_HOME"); d != "" {
			return filepath.Join(d, "NiraNode")
		}
		return filepath.Join(home, ".local", "share", "NiraNode")
	}
	return filepath.Join(home, "NiraNode")
}

// LoadInstall reads the bootstrap record; ok is false on a first run.
func LoadInstall() (Install, bool) {
	p, err := ConfigPath()
	if err != nil {
		return Install{}, false
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return Install{}, false
	}
	var in Install
	if json.Unmarshal(b, &in) != nil || in.DataDir == "" {
		return Install{}, false
	}
	if _, err := os.Stat(in.NiradPath); err != nil {
		return in, false // binary gone: install again into the same folder
	}
	return in, true
}

func saveInstall(in Install) error {
	p, err := ConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(in, "", "  ")
	return os.WriteFile(p, b, 0o600)
}

// AssetName is the release archive for this OS and CPU.
func AssetName(goos, goarch string) (string, error) {
	switch goos + "/" + goarch {
	case "linux/amd64", "linux/arm64", "darwin/amd64", "darwin/arm64":
		return fmt.Sprintf("nirad-%s-%s-%s.tar.gz", NiradVersion, goos, goarch), nil
	case "windows/amd64":
		return fmt.Sprintf("nirad-%s-windows-amd64.zip", NiradVersion), nil
	}
	return "", fmt.Errorf("no nirad build for %s/%s", goos, goarch)
}

// Progress reports download bytes (total may be -1 when unknown).
type Progress func(stage string, done, total int64)

// InstallNirad downloads this platform's nirad into dataDir/bin, verifies it
// against the release's SHA256SUMS, checks it runs, and records the install.
func InstallNirad(ctx context.Context, dataDir string, progress Progress) (Install, error) {
	if dataDir == "" || !filepath.IsAbs(dataDir) {
		return Install{}, errors.New("choose an absolute folder")
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "bin"), 0o700); err != nil {
		return Install{}, fmt.Errorf("cannot use %s: %w", dataDir, err)
	}
	asset, err := AssetName(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return Install{}, err
	}
	progress("Checking the release", 0, -1)
	sums, err := fetch(ctx, ReleaseBase()+"/SHA256SUMS", nil)
	if err != nil {
		return Install{}, fmt.Errorf("release checksums: %w", err)
	}
	want, err := checksumFor(sums, asset)
	if err != nil {
		return Install{}, err
	}
	archive, err := fetch(ctx, ReleaseBase()+"/"+asset, func(done, total int64) { progress("Downloading the node", done, total) })
	if err != nil {
		return Install{}, fmt.Errorf("download %s: %w", asset, err)
	}
	sum := sha256.Sum256(archive)
	if got := hex.EncodeToString(sum[:]); got != want {
		return Install{}, fmt.Errorf("checksum mismatch for %s: got %s, release says %s — not installed", asset, got, want)
	}
	progress("Verified, installing", int64(len(archive)), int64(len(archive)))
	name := "nirad"
	if runtime.GOOS == "windows" {
		name = "nirad.exe"
	}
	bin, err := extract(archive, asset, name)
	if err != nil {
		return Install{}, err
	}
	dst := filepath.Join(dataDir, "bin", name)
	tmp := dst + ".new"
	if err := os.WriteFile(tmp, bin, 0o755); err != nil {
		return Install{}, err
	}
	if err := os.Rename(tmp, dst); err != nil {
		return Install{}, err
	}
	out, err := exec.CommandContext(ctx, dst, "version").CombinedOutput()
	if err != nil {
		return Install{}, fmt.Errorf("installed nirad does not run: %v %s", err, out)
	}
	in := Install{DataDir: dataDir, NiradPath: dst, NiradVersion: strings.TrimSpace(string(out)), InstalledAt: time.Now().UTC()}
	return in, saveInstall(in)
}

func checksumFor(sums []byte, asset string) (string, error) {
	sc := bufio.NewScanner(bytes.NewReader(sums))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) == 2 && strings.TrimPrefix(f[1], "*") == asset && len(f[0]) == 64 {
			return strings.ToLower(f[0]), nil
		}
	}
	return "", fmt.Errorf("%s is not listed in the release's SHA256SUMS", asset)
}

const maxDownload = 512 << 20

func fetch(ctx context.Context, url string, onProgress func(done, total int64)) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	c := &http.Client{Timeout: 15 * time.Minute}
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s", resp.Status)
	}
	var buf bytes.Buffer
	r := io.LimitReader(resp.Body, maxDownload)
	chunk := make([]byte, 256<<10)
	var done int64
	for {
		n, err := r.Read(chunk)
		if n > 0 {
			buf.Write(chunk[:n])
			done += int64(n)
			if onProgress != nil {
				onProgress(done, resp.ContentLength)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

// extract returns the single file `name` from a .tar.gz or .zip archive.
func extract(archive []byte, asset, name string) ([]byte, error) {
	if strings.HasSuffix(asset, ".zip") {
		zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
		if err != nil {
			return nil, err
		}
		for _, f := range zr.File {
			if filepath.Base(f.Name) == name && !f.FileInfo().IsDir() {
				rc, err := f.Open()
				if err != nil {
					return nil, err
				}
				defer rc.Close()
				return io.ReadAll(io.LimitReader(rc, maxDownload))
			}
		}
		return nil, fmt.Errorf("%s not in %s", name, asset)
	}
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, err
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil, fmt.Errorf("%s not in %s", name, asset)
		}
		if err != nil {
			return nil, err
		}
		if filepath.Base(h.Name) == name && h.Typeflag == tar.TypeReg {
			return io.ReadAll(io.LimitReader(tr, maxDownload))
		}
	}
}
