package domain

import "testing"

func TestEWMACalc(t *testing.T) {
	tests := []struct {
		prevEWMA, sum float64
		n             int
		want          float64
	}{
		{0, 100, 7, 100 * 2.0 / 8},
		{50, 100, 7, 100*2.0/8 + 50*(1-2.0/8)},
		{0, 0, 0, 0},
	}
	for _, tt := range tests {
		got := EWMACalc(tt.prevEWMA, tt.sum, tt.n)
		if got != tt.want {
			t.Errorf("EWMACalc(%v, %v, %d) = %v; want %v", tt.prevEWMA, tt.sum, tt.n, got, tt.want)
		}
	}
}

func TestComputeACWR(t *testing.T) {
	loads := []float64{100, 120, 130, 140, 150, 160, 170, 180, 190, 200}
	result := ComputeACWR(loads)

	if result.Acute <= 0 {
		t.Error("acute load should be positive")
	}
	if result.Chronic <= 0 {
		t.Error("chronic load should be positive")
	}
	if result.Acwr <= 0 {
		t.Error("acwr should be positive")
	}
	if result.Zone != "green" && result.Zone != "yellow" && result.Zone != "red" {
		t.Errorf("unexpected zone: %s", result.Zone)
	}
}

func TestComputeACWRSpike(t *testing.T) {
	loads := []float64{100, 100, 100, 100, 100, 100, 100, 300}
	result := ComputeACWR(loads)

	if result.Zone != "red" {
		t.Errorf("expected red zone for spike, got %s", result.Zone)
	}
}
