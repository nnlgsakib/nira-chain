package cli

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// OnionFile is the community node's local onion secret. It is not a funds
// key, but leaking it lets others predict this node's selections, so it is
// written with 0600 permissions and never sent anywhere.
type OnionFile struct {
	Secret string `json:"secret"` // hex, 32 bytes
	Length uint32 `json:"length"`
	// BaseEpoch is filled after registration / rotation from the chain.
	BaseEpoch uint64 `json:"base_epoch,omitempty"`
}

func onionPath(home string) string { return filepath.Join(home, "community", "onion.json") }

func hashN(v []byte, n uint64) []byte {
	for i := uint64(0); i < n; i++ {
		s := sha256.Sum256(v)
		v = s[:]
	}
	return v
}

func newOnion(length uint32) (OnionFile, error) {
	s := make([]byte, 32)
	if _, err := rand.Read(s); err != nil {
		return OnionFile{}, err
	}
	return OnionFile{Secret: hex.EncodeToString(s), Length: length}, nil
}

func (o OnionFile) secret() []byte { b, _ := hex.DecodeString(o.Secret); return b }

// Tip = H^K(s).
func (o OnionFile) Tip() []byte { return hashN(o.secret(), uint64(o.Length)) }

// Layer returns v_e = H^{K-(e-base+1)}(s), or an error if e is outside the onion.
func (o OnionFile) Layer(e, base uint64) ([]byte, error) {
	if e < base || e-base+1 > uint64(o.Length) {
		return nil, fmt.Errorf("epoch %d outside onion (base %d, length %d): rotate the onion", e, base, o.Length)
	}
	return hashN(o.secret(), uint64(o.Length)-(e-base+1)), nil
}

func saveOnion(home string, o OnionFile, overwrite bool) error {
	p := onionPath(home)
	if _, err := os.Stat(p); err == nil && !overwrite {
		return fmt.Errorf("%s exists (use --overwrite to replace; the old onion becomes unusable)", p)
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	bz, _ := json.MarshalIndent(o, "", "  ")
	return os.WriteFile(p, bz, 0o600)
}

func loadOnion(home string) (OnionFile, error) {
	var o OnionFile
	bz, err := os.ReadFile(onionPath(home))
	if err != nil {
		return o, fmt.Errorf("read onion (run `community onion-init` first): %w", err)
	}
	return o, json.Unmarshal(bz, &o)
}
