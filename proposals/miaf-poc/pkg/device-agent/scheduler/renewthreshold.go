// Package scheduler - wall-clock cadences for daemon background jobs.
package scheduler

import "time"

// RenewAt returns the moment at which the daemon SHOULD attempt renewal,
// given the leaf's notBefore + notAfter and the operator-chosen ratio.
// Default ratio is 0.5 (50% lifetime).
func RenewAt(notBefore, notAfter time.Time, ratio float64) time.Time {
	if ratio <= 0 || ratio >= 1 {
		ratio = 0.5
	}
	lifetime := notAfter.Sub(notBefore)
	delta := time.Duration(float64(lifetime) * ratio)
	return notBefore.Add(delta)
}
