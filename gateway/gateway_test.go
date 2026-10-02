package gateway

import "testing"

func TestName(t *testing.T) {
	if Name != "gateway" {
		t.Fatalf("unexpected name %q", Name)
	}
}
