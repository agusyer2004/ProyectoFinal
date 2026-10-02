package sink

import "testing"

func TestName(t *testing.T) {
	if Name != "sink" {
		t.Fatalf("unexpected name %q", Name)
	}
}
