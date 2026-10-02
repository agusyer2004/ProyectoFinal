package event_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/agusyer2004/ProyectoFinal/gateway/event"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

const (
	schemaID    = "https://anomalous.local/schema/event.v1.json"
	schemaFile  = "../../schema/event.v1.schema.json"
	fixturesDir = "../../schema/fixtures"
)

var eventTypes = []string{
	event.TypeLoginAttempt, event.TypeAccountCreated, event.TypeCartAction,
	event.TypeTransaction, event.TypeRead, event.TypeTransactionOutcome,
}

func compile(t *testing.T, fragment string) *jsonschema.Schema {
	t.Helper()
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(readFile(t, schemaFile)))
	if err != nil {
		t.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft2020)
	c.AssertFormat()
	if err := c.AddResource(schemaID, doc); err != nil {
		t.Fatal(err)
	}
	s, err := c.Compile(schemaID + fragment)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func decode(t *testing.T, raw []byte) any {
	t.Helper()
	v, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func fixtures(t *testing.T, sub string) []string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(fixturesDir, sub, "*.json"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("sin fixtures en %s: %v", sub, err)
	}
	return paths
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestFixtures(t *testing.T) {
	cases := []struct {
		sub      string
		fragment string
		valid    bool
	}{
		{"valid/client", "#/$defs/ClientEvent", true},
		{"valid/event", "", true},
		{"invalid/client", "#/$defs/ClientEvent", false},
		{"invalid/event", "", false},
	}
	for _, tc := range cases {
		schema := compile(t, tc.fragment)
		for _, path := range fixtures(t, tc.sub) {
			t.Run(tc.sub+"/"+filepath.Base(path), func(t *testing.T) {
				err := schema.Validate(decode(t, readFile(t, path)))
				if tc.valid && err != nil {
					t.Fatalf("debería ser válido: %v", err)
				}
				if !tc.valid && err == nil {
					t.Fatal("debería ser inválido")
				}
			})
		}
	}
}

func TestEveryEventTypeHasValidFixture(t *testing.T) {
	for _, sub := range []string{"valid/client", "valid/event"} {
		for _, typ := range eventTypes {
			if _, err := os.Stat(filepath.Join(fixturesDir, sub, typ+".json")); err != nil {
				t.Errorf("falta fixture %s/%s: %v", sub, typ, err)
			}
		}
	}
}

// Ida y vuelta: fixture -> structs de Go -> JSON tiene que seguir validando contra el esquema.
func TestRoundTrip(t *testing.T) {
	schema := compile(t, "")
	for _, typ := range eventTypes {
		t.Run(typ, func(t *testing.T) {
			raw := readFile(t, filepath.Join(fixturesDir, "valid/event", typ+".json"))

			var e1 event.Event
			if err := json.Unmarshal(raw, &e1); err != nil {
				t.Fatalf("decodificar fixture: %v", err)
			}
			out, err := json.Marshal(e1)
			if err != nil {
				t.Fatal(err)
			}
			if err := schema.Validate(decode(t, out)); err != nil {
				t.Fatalf("lo que serializa Go no valida: %v\n%s", err, out)
			}

			var e2 event.Event
			if err := json.Unmarshal(out, &e2); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(e1, e2) {
				t.Fatalf("la ida y vuelta cambió el evento:\n%+v\n%+v", e1, e2)
			}
		})
	}
}

// Los structs también deben rechazar lo que el esquema rechaza por campos desconocidos (ADR-004).
func TestUnmarshalRejectsFraudLabels(t *testing.T) {
	raw := readFile(t, filepath.Join(fixturesDir, "invalid/event", "extra_field_is_fraud.json"))
	var e event.Event
	if err := json.Unmarshal(raw, &e); err == nil {
		t.Fatal("is_fraud no debería decodificarse")
	}
}
