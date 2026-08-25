package clock

import (
	"testing"
	"time"
)

func TestReal_NowMonotonic(t *testing.T) {
	t.Parallel()
	c := Real{}
	a := c.Now()
	time.Sleep(time.Millisecond)
	b := c.Now()
	if !b.After(a) {
		t.Errorf("Real clock did not advance: a=%v b=%v", a, b)
	}
}

func TestFake_AdvanceFiresTimers(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	c := NewFake(start)

	timer := c.NewTimer(5 * time.Minute)
	select {
	case <-timer.C():
		t.Fatal("timer fired before Advance")
	default:
	}

	c.Advance(5*time.Minute - 1)
	select {
	case <-timer.C():
		t.Fatal("timer fired before its full duration")
	default:
	}

	c.Advance(time.Second)
	select {
	case fired := <-timer.C():
		if !fired.Equal(start.Add(5 * time.Minute)) {
			t.Errorf("fired at %v, want %v", fired, start.Add(5*time.Minute))
		}
	case <-time.After(time.Second):
		t.Fatal("timer did not fire after Advance past its deadline")
	}
}

func TestFake_NowReadsAdvancedTime(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	c := NewFake(start)
	c.Advance(time.Hour)
	if got := c.Now(); !got.Equal(start.Add(time.Hour)) {
		t.Errorf("Now=%v, want %v", got, start.Add(time.Hour))
	}
}
