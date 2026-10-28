package validator

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"sigs.k8s.io/yaml"
)

const (
	chartDir = "../../charts/trustify"
	// The values-*.yaml files live at the repository root.
	valuesIn = "../.."
)

// appDomain is supplied on the command line in every documented install, so
// the values files do not carry it and would not validate without it.
const appDomain = ".localhost"

// cases mirror the install commands in README.md. Keeping them in step is the
// point: a values file that the README tells people to use must validate.
var cases = []struct {
	name  string
	files []string
}{
	{"minikube", []string{"values-minikube.yaml"}},
	{"ocp-no-aws", []string{"values-ocp-no-aws.yaml"}},
	{"ocp-aws", []string{"values-ocp-aws.yaml"}},
	// README: "--values values-ocp-no-aws.yaml ... --values values-crc.yaml".
	// values-crc.yaml is an overlay and does not stand on its own.
	{"ocp-no-aws+crc", []string{"values-ocp-no-aws.yaml", "values-crc.yaml"}},
	// README: "create an initial set of importers by adding values-importers.yaml".
	// This is the case that actually exercises urn:trustify:importer.
	{"minikube+importers", []string{"values-minikube.yaml", "values-importers.yaml"}},
	{"ocp-no-aws+importers", []string{"values-ocp-no-aws.yaml", "values-importers.yaml"}},
}

func registerOrFail(t *testing.T) {
	t.Helper()
	saved := chartutil.URNResolver
	t.Cleanup(func() { chartutil.URNResolver = saved })
	if _, err := Register(SchemasDir); err != nil {
		t.Fatalf("Register: %v", err)
	}
}

// merge layers values files the way `helm -f a -f b` does: later wins.
func merge(base, overlay map[string]any) map[string]any {
	out := make(map[string]any, len(base))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range overlay {
		if sub, ok := v.(map[string]any); ok {
			if existing, ok := out[k].(map[string]any); ok {
				out[k] = merge(existing, sub)
				continue
			}
		}
		out[k] = v
	}
	return out
}

func readValues(t *testing.T, names []string) map[string]any {
	t.Helper()
	merged := map[string]any{"appDomain": appDomain}
	for _, name := range names {
		raw, err := os.ReadFile(filepath.Join(valuesIn, name))
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		var doc map[string]any
		if err := yaml.Unmarshal(raw, &doc); err != nil {
			t.Fatalf("parsing %s: %v", name, err)
		}
		merged = merge(merged, doc)
	}
	return merged
}

func validate(t *testing.T, values map[string]any) error {
	t.Helper()
	chrt, err := loader.Load(chartDir)
	if err != nil {
		t.Fatalf("loading chart: %v", err)
	}
	coalesced, err := chartutil.CoalesceValues(chrt, values)
	if err != nil {
		t.Fatalf("coalescing values: %v", err)
	}
	return chartutil.ValidateAgainstSchema(chrt, coalesced)
}

func TestRegisteredURNs(t *testing.T) {
	docs, err := Load(SchemasDir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := []string{
		"urn:k8s:v1.29.0:EnvVarSource",
		"urn:k8s:v1.29.0:Quantity",
		"urn:k8s:v1.29.0:Volume",
		"urn:k8s:v1.29.0:VolumeMount",
		"urn:trustify:auth",
		"urn:trustify:importer",
	}
	if got := URNs(docs); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("registered URNs:\n got %v\nwant %v", got, want)
	}
}

// The chart must not reach the network to validate values -- that is the
// whole reason the refs are urn:, and it is one edit away from regressing.
func TestChartHasNoRemoteRefs(t *testing.T) {
	docs, err := Load(SchemasDir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	remote := regexp.MustCompile(`\$ref"?\s*:\s*"(https?://[^"]+)"`)
	urnRef := regexp.MustCompile(`"(urn:[^"]*)"`)

	for _, name := range []string{"values.schema.json", "values.schema.yaml"} {
		body, err := os.ReadFile(filepath.Join(chartDir, name))
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		for _, m := range remote.FindAllStringSubmatch(string(body), -1) {
			t.Errorf("%s: remote $ref reintroduced: %s", name, m[1])
		}
		for _, m := range urnRef.FindAllStringSubmatch(string(body), -1) {
			urn := m[1]
			// A URN with a JSON-pointer fragment breaks the offline
			// fallback: Helm substitutes a bare `true` schema for an
			// unregistered URN and `#/...` cannot resolve against it.
			if strings.Contains(urn, "#") {
				t.Errorf("%s: %s has a JSON-pointer fragment", name, urn)
				continue
			}
			if _, ok := docs[urn]; !ok {
				t.Errorf("%s: %s is referenced but not vendored", name, urn)
			}
		}
	}
}

// Every values file the README tells people to install with must validate
// against the real schemas, not the permissive fallback.
func TestDocumentedValuesValidate(t *testing.T) {
	registerOrFail(t)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := validate(t, readValues(t, tc.files)); err != nil {
				t.Errorf("%s rejected by the chart schema:\n%v", strings.Join(tc.files, " + "), err)
			}
		})
	}
}

// Registering must actually tighten validation. Without this the whole
// mechanism could be inert and every test above would still pass.
func TestRegisteredSchemasAreEnforced(t *testing.T) {
	registerOrFail(t)

	// `period` is a trustify HumantimeSerde (a string); an object is not
	// valid. This can only be rejected if urn:trustify:importer resolved.
	values := readValues(t, []string{"values-minikube.yaml"})
	values["modules"] = map[string]any{
		"createImporters": map[string]any{
			"importers": map[string]any{
				"bad": map[string]any{
					"sbom": map[string]any{
						"source": "https://example.com",
						"period": map[string]any{"not": "a duration"},
					},
				},
			},
		},
	}
	err := validate(t, values)
	if err == nil {
		t.Fatal("invalid importer config accepted: urn:trustify:importer was not enforced")
	}
	if !strings.Contains(err.Error(), "importers") {
		t.Errorf("rejected, but not for the importer: %v", err)
	}
}

// The Kubernetes URNs must be enforced too, not just the trustify ones.
func TestKubernetesSchemasAreEnforced(t *testing.T) {
	registerOrFail(t)

	values := readValues(t, []string{"values-minikube.yaml"})
	values["extraVolumes"] = []any{
		map[string]any{"name": 12345, "configMap": map[string]any{"name": "cm"}},
	}
	if err := validate(t, values); err == nil {
		t.Fatal("volume with a non-string name accepted: urn:k8s:v1.29.0:Volume was not enforced")
	}
}

// Without a resolver the chart must still validate, degrading to permissive
// rather than failing -- that is what makes a plain `helm install` work
// offline, and it is the reason the refs are urn: and not remote.
func TestUnregisteredURNsDegradeGracefully(t *testing.T) {
	saved := chartutil.URNResolver
	t.Cleanup(func() { chartutil.URNResolver = saved })
	chartutil.URNResolver = func(urn string) (any, error) {
		return nil, errUnresolved(urn)
	}
	if err := validate(t, readValues(t, []string{"values-minikube.yaml"})); err != nil {
		t.Fatalf("valid values rejected with no resolver registered: %v", err)
	}
}

type errUnresolved string

func (e errUnresolved) Error() string { return "unresolved: " + string(e) }
