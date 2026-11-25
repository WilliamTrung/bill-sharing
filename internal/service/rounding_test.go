package service

import (
	"math/rand"
	"testing"
)

// Largest-remainder inputs where the total does not divide evenly.
func TestAllocateEvenSplitGivesFirstShareTheRemainder(t *testing.T) {
	cases := []struct {
		total int64
		want  []int64
	}{
		{100000, []int64{33334, 33333, 33333}},
		{100, []int64{34, 33, 33}},
		{10, []int64{4, 3, 3}},
		{100000, []int64{25000, 25000, 25000, 25000}},
	}
	for _, tc := range cases {
		got, err := Allocate(tc.total, EvenWeights(len(tc.want)))
		if err != nil {
			t.Fatalf("Allocate(%d): %v", tc.total, err)
		}
		for i := range tc.want {
			if got[i] != tc.want[i] {
				t.Fatalf("Allocate(%d, even) = %v, want %v", tc.total, got, tc.want)
			}
		}
	}
}

// A user-typed decimal (e.g. "1000.00" -> 100000 cents) must never end up as a
// fractional cent: the big-int path only ever yields integers.
func TestAllocateProducesOnlyIntegerCents(t *testing.T) {
	got, err := Allocate(100000, []float64{1.0, 1.0, 1.0})
	if err != nil {
		t.Fatal(err)
	}
	if sum(got) != 100000 {
		t.Fatalf("sum = %d, want 100000 (shares %v)", sum(got), got)
	}
	if got[0] != 33334 || got[1] != 33333 || got[2] != 33333 {
		t.Fatalf("got %v, want [33334 33333 33333]", got)
	}
}

// Fractional weights must still allocate whole cents and conserve the total.
func TestAllocateHandlesFractionalWeights(t *testing.T) {
	got, err := Allocate(100000, []float64{1.5, 1.5, 1})
	if err != nil {
		t.Fatal(err)
	}
	if sum(got) != 100000 {
		t.Fatalf("sum = %d, want 100000", sum(got))
	}
	// 1.5:1.5:1 of 100000 is exactly 37500/37500/25000.
	if got[0] != 37500 || got[1] != 37500 || got[2] != 25000 {
		t.Fatalf("got %v, want [37500 37500 25000]", got)
	}
}

func TestAllocateRemainderDistributionIsDeterministic(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	for i := 0; i < 500; i++ {
		total := rng.Int63n(100000) + 1
		count := rng.Intn(6) + 2

		a, err := Allocate(total, EvenWeights(count))
		if err != nil {
			t.Fatalf("Allocate(%d, %d): %v", total, count, err)
		}
		b, err := Allocate(total, EvenWeights(count))
		if err != nil {
			t.Fatal(err)
		}
		for j := range a {
			if a[j] != b[j] {
				t.Fatalf("Allocate(%d, %d) not deterministic: %v vs %v", total, count, a, b)
			}
			if a[j] < 0 {
				t.Fatalf("negative share: %v", a)
			}
			// Never more than one cent apart for equal weights.
			if a[j]-a[0] > 1 || a[0]-a[j] > 1 {
				t.Fatalf("shares differ by more than one cent: %v", a)
			}
		}
	}
}

// Guard against float creep turning 1.0 into something that is not exactly one
// weight unit after scaling.
func TestAllocateScalesWeightsExactly(t *testing.T) {
	weights := []float64{0.1, 0.2, 0.7}
	got, err := Allocate(1000, weights)
	if err != nil {
		t.Fatal(err)
	}
	if sum(got) != 1000 {
		t.Fatalf("sum = %d, want 1000", sum(got))
	}
	want := []int64{100, 200, 700}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}
