package fixtures

import "testing"

func TestPlantedDuplicateLiteral(t *testing.T) {
	if got := len(PlantedDuplicateLiteral()); got != 4 {
		t.Fatalf("got %d, want 4", got)
	}
}
