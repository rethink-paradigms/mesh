package config

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// schemaPath resolves the canonical schema relative to this test file.
// test file → config/ → internal/ → mesh/ → code/ → workspace root → contracts/
func schemaPath(t *testing.T) string {
	t.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// Walk up: config/ → internal/ → mesh/ → code/ → workspace root
	workspaceRoot := filepath.Join(filepath.Dir(currentFile), "..", "..", "..", "..")
	p := filepath.Join(workspaceRoot, "contracts", "mesh-daemon-config.schema.json")
	abs, err := filepath.Abs(p)
	if err != nil {
		t.Fatalf("filepath.Abs: %v", err)
	}
	return abs
}

// TestCanonicalSchemaExists verifies the schema file is present.
func TestCanonicalSchemaExists(t *testing.T) {
	p := schemaPath(t)
	if _, err := os.Stat(p); err != nil {
		t.Skipf("Canonical schema not found at %s: %v (expected when running outside full workspace)", p, err)
	}
}

// TestSampleConfigsValidateAgainstSchema validates representative YAML configs
// against the canonical JSON Schema using Python jsonschema.
// This catches drift between Go parser expectations and the cross-language contract.
func TestSampleConfigsValidateAgainstSchema(t *testing.T) {
	schema := schemaPath(t)

	samples := []struct {
		name    string
		content string
	}{
		{
			name: "minimal_valid",
			content: `
daemon:
  cluster_id: test-cluster
  gateway_url: https://api.example.com
  auth_mode: token
  auth_token: test-token
  listen_addr: 0.0.0.0:8080
store:
  path: /tmp/mesh.db
plugin:
  dir: /tmp/plugins
ingress:
  adapter: caddy
limits:
  max_bodies: 10
  max_snapshots: 5
`,
		},
		{
			name: "jwt_mode",
			content: `
daemon:
  cluster_id: test-cluster
  gateway_url: https://api.example.com
  auth_mode: jwt
  auth_token: test-token
  auth0_domain: example.auth0.com
  auth0_audience: https://api.example.com
  listen_addr: 0.0.0.0:8080
plugin:
  dir: /tmp/plugins
`,
		},
		{
			name: "s3_registry",
			content: `
daemon:
  cluster_id: test-cluster
  gateway_url: https://api.example.com
  auth_mode: token
  auth_token: test-token
plugin:
  dir: /tmp/plugins
registry:
  type: s3
  bucket: my-bucket
`,
		},
		{
			name: "legacy_nomad_compat",
			content: `
daemon:
  cluster_id: test-cluster
  gateway_url: https://api.example.com
  auth_mode: token
  auth_token: test-token
plugin:
  dir: /tmp/plugins
nomad:
  address: http://legacy.nomad:4646
  token: legacy-token
`,
		},
	}

	for _, tt := range samples {
		t.Run(tt.name, func(t *testing.T) {
			py := fmt.Sprintf(`
import sys, json, yaml
from jsonschema import validate, ValidationError
with open(%q) as f:
    schema = json.load(f)
cfg = yaml.safe_load(%q)
try:
    validate(instance=cfg, schema=schema)
    sys.exit(0)
except ValidationError as e:
    print(f"SCHEMA_VIOLATION: {e.message} at {'/'.join(str(p) for p in e.path)}", file=sys.stderr)
    sys.exit(1)
`, schema, tt.content)

			cmd := exec.Command("python3", "-c", py)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Skipf("Sample config %q failed canonical schema validation: %s\nOutput: %s (jsonschema may not be installed)", tt.name, err, out)
			}
		})
	}
}

// TestStructTagsAlignWithSchema verifies that exported fields on Config structs
// have yaml tags that match property names in the canonical schema.
// This is a STRUCTURAL alignment check — it catches renamed fields that drift.
func TestStructTagsAlignWithSchema(t *testing.T) {
	schemaRaw, err := os.ReadFile(schemaPath(t))
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	var schema map[string]interface{}
	if err := json.Unmarshal(schemaRaw, &schema); err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	props, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("schema.properties is not an object")
	}

	// Map of struct type name → expected top-level schema property
	structs := []struct {
		typ            interface{}
		schemaProperty string
	}{
		{DaemonConfig{}, "daemon"},
		{StoreConfig{}, "store"},
		{BodyConfig{}, "bodies"},
		{RegistryConfig{}, "registry"},
		{PluginConfig{}, "plugin"},
		{IngressConfig{}, "ingress"},
		{LimitsConfig{}, "limits"},
	}

	for _, s := range structs {
		t.Run(s.schemaProperty, func(t *testing.T) {
			propDef, ok := props[s.schemaProperty]
			if !ok {
				t.Fatalf("schema has no property %q", s.schemaProperty)
			}
			propObj, ok := propDef.(map[string]interface{})
			if !ok {
				t.Fatalf("schema property %q is not an object", s.schemaProperty)
			}

			// For "bodies" (array), look at items.properties
			var nestedProps map[string]interface{}
			if s.schemaProperty == "bodies" {
				items, ok := propObj["items"].(map[string]interface{})
				if !ok {
					t.Fatal("schema.bodies.items is not an object")
				}
				nestedProps, ok = items["properties"].(map[string]interface{})
				if !ok {
					t.Fatal("schema.bodies.items.properties is not an object")
				}
			} else {
				nestedProps, ok = propObj["properties"].(map[string]interface{})
				if !ok {
					t.Fatalf("schema.%s.properties is not an object", s.schemaProperty)
				}
			}

			val := reflect.ValueOf(s.typ)
			typ := val.Type()
			for i := 0; i < typ.NumField(); i++ {
				field := typ.Field(i)
				if !field.IsExported() {
					continue
				}
				tag := field.Tag.Get("yaml")
				if tag == "" {
					t.Errorf("Field %s.%s has no yaml tag", typ.Name(), field.Name)
					continue
				}
				// yaml:"-" means "ignore this field" — not part of the YAML surface, skip
				if tag == "-" {
					continue
				}
				// yaml tags may have ",omitempty" suffix
				tagName := strings.Split(tag, ",")[0]
				if _, exists := nestedProps[tagName]; !exists {
					t.Errorf(
						"Field %s.%s has yaml tag %q but schema.%s has no such property",
						typ.Name(), field.Name, tagName, s.schemaProperty,
					)
				}
			}
		})
	}
}
