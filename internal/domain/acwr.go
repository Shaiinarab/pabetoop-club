// Package domain contains pure business math (ACWR, PHV, load aggregation) that
// has no PocketBase dependency — unit-testable in isolation.
package domain

import "math"

// EWMACalc computes an exponentially weighted moving average.
// alpha = 2 / (N + 1), where N is the window size (7 for acute, 28 for chronic).
func EWMACalc(prevEWMA, sum float64, n int) float64 {
	if n == 0 {
		return 0
	}
	alpha := 2.0 / float64(n+1)
	return alpha*sum + (1-alpha)*prevEWMA
}

// ACWRResult holds the computed acute/chronic load and ratio.
type ACWRResult struct {
	Acute      float64
	Chronic    float64
	Acwr       float64
	SpikeDelta float64
	Zone       string // green | yellow | red
}

// ComputeACWR calculates the acute:chronic workload ratio from a slice of daily
// training loads ordered oldest→newest. Returns a zone verdict.
func ComputeACWR(loads []float64) ACWRResult {
	var acute, chronic float64
	var prevAcute, spike float64

	for i, load := range loads {
		previousAcute := acute
		if i < 7 {
			acute = EWMACalc(0, load, i+1)
		} else {
			acute = EWMACalc(prevAcute, load, 7)
		}
		if i < 28 {
			chronic = EWMACalc(0, load, i+1)
		} else {
			windowSum := 0.0
			for j := i - 27; j <= i; j++ {
				windowSum += loads[j]
			}
			chronic = EWMACalc(chronic, windowSum, 28)
		}
		if i == len(loads)-1 && previousAcute > 0 {
			spike = (acute - previousAcute) / previousAcute
		}
		prevAcute = acute
	}

	acwr := 0.0
	if chronic > 0 {
		acwr = acute / chronic
	}

	zone := "green"
	if acwr > 2.5 || acwr < 0.8 || math.Abs(spike) > 0.15 {
		zone = "red"
	} else if acwr > 1.5 || acwr < 0.9 || math.Abs(spike) > 0.08 {
		zone = "yellow"
	}

	return ACWRResult{
		Acute:      acute,
		Chronic:    chronic,
		Acwr:       acwr,
		SpikeDelta: spike,
		Zone:       zone,
	}
}
