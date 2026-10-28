// Package validator registers the schemas that charts/trustify's values
// schema refers to by `urn:`, so this repository's values files can be
// validated in CI.
//
// The chart refers to the trustify importer and auth configs, and four
// Kubernetes core types, as `urn:` rather than over the network -- remote
// `$ref`s made `helm template`/`install` fail outright in an air-gapped
// cluster. Helm resolves that scheme through the pluggable
// chartutil.URNResolver, and an unresolved URN is deliberately not fatal:
// Helm logs a warning and substitutes a permissive schema. That is what keeps
// a plain `helm install` of this chart working offline.
//
// The cost is that the helm CLI never validates those six subtrees, because
// nothing registers a resolver for it. This package closes that gap for CI
// only: it plugs in the copies vendored under hack/schemas so the values
// files in this repository are checked against the real schemas. The chart is
// untouched, which is what keeps it byte-identical to the operator's copy.
package validator

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"helm.sh/helm/v3/pkg/chartutil"
)

// SchemasDir is the vendored schema directory, relative to this package.
const SchemasDir = "../schemas"

const sourcesFile = "SOURCES.json"

type sources struct {
	Schemas map[string]struct {
		URN    string `json:"urn"`
		SHA256 string `json:"sha256"`
	} `json:"schemas"`
}

// Load reads the vendored schemas and returns them keyed by URN.
//
// Each file is checked against the digest recorded in SOURCES.json, so a copy
// edited by hand rather than re-vendored is caught here instead of quietly
// changing what the values files are validated against.
func Load(dir string) (map[string]any, error) {
	raw, err := os.ReadFile(filepath.Join(dir, sourcesFile))
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", sourcesFile, err)
	}
	var src sources
	if err := json.Unmarshal(raw, &src); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", sourcesFile, err)
	}
	if len(src.Schemas) == 0 {
		return nil, fmt.Errorf("%s declares no schemas", sourcesFile)
	}

	// Every vendored file must be accounted for in SOURCES.json; an
	// added-but-unlisted schema would silently never be registered.
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", dir, err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if name == sourcesFile || filepath.Ext(name) != ".json" {
			continue
		}
		if _, ok := src.Schemas[name]; !ok {
			return nil, fmt.Errorf("%s: vendored but not listed in %s", name, sourcesFile)
		}
	}

	out := make(map[string]any, len(src.Schemas))
	for name, meta := range src.Schemas {
		body, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", name, err)
		}
		sum := sha256.Sum256(body)
		if digest := hex.EncodeToString(sum[:]); digest != meta.SHA256 {
			return nil, fmt.Errorf(
				"%s: sha256 %s does not match %s (%s); re-run hack/vendor-values-schemas.py --fetch",
				name, digest, sourcesFile, meta.SHA256)
		}
		if meta.URN == "" {
			return nil, fmt.Errorf("%s: no urn in %s", name, sourcesFile)
		}
		// UnmarshalJSON rather than encoding/json: the validator needs
		// numbers kept as json.Number for the comparisons it does.
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("parsing %s: %w", name, err)
		}
		if _, dup := out[meta.URN]; dup {
			return nil, fmt.Errorf("%s: urn %s is claimed by more than one schema", name, meta.URN)
		}
		out[meta.URN] = doc
	}
	return out, nil
}

// URNs lists the registered URNs, sorted.
func URNs(docs map[string]any) []string {
	urns := make([]string, 0, len(docs))
	for urn := range docs {
		urns = append(urns, urn)
	}
	sort.Strings(urns)
	return urns
}

// Register installs the vendored schemas as Helm's URN resolver.
//
// This mutates package-level state in Helm, the only hook it exposes for
// this, so it must happen before any chart is validated.
func Register(dir string) ([]string, error) {
	docs, err := Load(dir)
	if err != nil {
		return nil, err
	}
	chartutil.URNResolver = func(urn string) (any, error) {
		doc, ok := docs[urn]
		if !ok {
			// An error here lets Helm apply its permissive fallback, which
			// is the right behaviour for a URN this build does not know.
			return nil, fmt.Errorf("no vendored schema for %s", urn)
		}
		return doc, nil
	}
	return URNs(docs), nil
}
