package stack

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	"bws/internal/config"

	"github.com/tailscale/hujson"
)

var validName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)

// Provenance records creation origin and verification digest.
type Provenance struct {
	SourceWorkspace string    `json:"source_workspace,omitempty"`
	SavedAt         time.Time `json:"saved_at,omitempty"`
	VerifiedDigest  string    `json:"verified_digest,omitempty"`
}

// Stack defines a high-level developer persona and environment baseline.
type Stack struct {
	Name        string                 `json:"name"`
	Title       string                 `json:"title,omitempty"`
	Description string                 `json:"description,omitempty"`
	Category    string                 `json:"category,omitempty"`
	Profiles    []string               `json:"profiles"`
	DefaultCmd  []string               `json:"default_cmd,omitempty"`
	Features    *config.FeaturesConfig `json:"features,omitempty"`
	Env         map[string]string      `json:"env,omitempty"`
	Provenance  *Provenance            `json:"provenance,omitempty"`
	Source      string                 `json:"source,omitempty"` // "embedded" or user file path
}

// ValidateName ensures the stack name is a clean, alphanumeric identifier without path traversal.
func ValidateName(name string) error {
	if !validName.MatchString(name) || len(name) > 100 {
		return fmt.Errorf("invalid stack name %q: use letters, digits, dots, underscores or hyphens", name)
	}
	return nil
}

// Validate ensures required fields and constituent profile names are valid.
func Validate(s *Stack) error {
	if s == nil {
		return fmt.Errorf("stack definition is nil")
	}
	if err := ValidateName(s.Name); err != nil {
		return err
	}
	if len(s.Profiles) == 0 {
		return fmt.Errorf("stack %q must contain at least one profile", s.Name)
	}
	for _, p := range s.Profiles {
		if !validName.MatchString(p) {
			return fmt.Errorf("stack %q has invalid profile reference %q", s.Name, p)
		}
	}
	return nil
}

// Digest computes a deterministic SHA-256 hash of the canonical stack definition.
func Digest(s *Stack) (string, error) {
	if s == nil {
		return "", fmt.Errorf("cannot compute digest of nil stack")
	}
	cp := *s
	cp.Provenance = nil
	cp.Source = ""
	data, err := json.Marshal(cp)
	if err != nil {
		return "", fmt.Errorf("marshaling stack for digest: %w", err)
	}
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum), nil
}

// Fingerprint returns a ProfileApproval record reflecting the stack source and SHA-256 digest.
func Fingerprint(s *Stack) (config.ProfileApproval, error) {
	digest, err := Digest(s)
	if err != nil {
		return config.ProfileApproval{}, err
	}
	return config.ProfileApproval{
		Source: s.Source,
		SHA256: digest,
	}, nil
}

// Parse parses JSON or JSONC data into a validated Stack.
func Parse(data []byte) (*Stack, error) {
	standardized, err := hujson.Standardize(data)
	if err != nil {
		return nil, fmt.Errorf("standardizing JSONC: %w", err)
	}
	var s Stack
	if err := json.Unmarshal(standardized, &s); err != nil {
		return nil, fmt.Errorf("parsing stack: %w", err)
	}
	if err := Validate(&s); err != nil {
		return nil, err
	}
	return &s, nil
}

// ToJSON serializes the stack definition to formatted JSON.
func ToJSON(s *Stack) ([]byte, error) {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}
