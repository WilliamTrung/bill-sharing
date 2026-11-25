package service

import (
	"math/rand"
	"testing"
)

func sum(shares []int64) int64 {
	var total int64
	for _, share := range shares {
		total += share
	}
	return total
}

func TestAllocateEvenSplitsAreExact(t *testing.T) {
	cases := []struct {
		total   int64
		weights []float64
		want    []int64
	}{
		{10000, []float64{1, 1, 1}, []int64{3334, 3333, 3333}},
		{100, []float64{1, 1, 1}, []int64{34, 33, 33}},
		{5, []float64{1, 1, 1}, []int64{2, 2, 1}},
		{10000, []float64{2, 1, 1}, []int64{5000, 2500, 2500}},
		{1000, []float64{1}, []int64{1000}},
		{1, []float64{1, 1}, []int64{1, 0}},
	}

	for _, tc := range cases {
		got, err := Allocate(tc.total, tc.weights)
		if err != nil {
			t.Fatalf("Allocate(%d, %v): %v", tc.total, tc.weights, err)
		}
		if sum(got) != tc.total {
			t.Fatalf("Allocate(%d, %v) = %v, sum %d != %d", tc.total, tc.weights, got, sum(got), tc.total)
		}
		for i := range tc.want {
			if got[i] != tc.want[i] {
				t.Fatalf("Allocate(%d, %v) = %v, want %v", tc.total, tc.weights, got, tc.want)
			}
		}
	}
}

func TestAllocateAlwaysSumsToTotal(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	for i := 0; i < 2000; i++ {
		total := rng.Int63n(maxTotalCents) + 1
		count := rng.Intn(8) + 1
		weights := make([]float64, count)
		for j := range weights {
			weights[j] = float64(rng.Intn(1000)+1) / 10
		}
		got, err := Allocate(total, weights)
		if err != nil {
			t.Fatalf("Allocate(%d, %v): %v", total, weights, err)
		}
		if sum(got) != total {
			t.Fatalf("Allocate(%d, %v) sums to %d", total, weights, sum(got))
		}
		for _, share := range got {
			if share < 0 {
				t.Fatalf("Allocate(%d, %v) produced a negative share: %v", total, weights, got)
			}
		}
	}
}

func TestAllocateRemainderGoesToLargestRemainder(t *testing.T) {
	got, err := Allocate(10, []float64{1, 1, 1})
	if err != nil {
		t.Fatal(err)
	}
	if got[0] != 4 {
		t.Fatalf("expected first share to receive the remainder, got %v", got)
	}
}

func TestAllocateRejectsInvalidInput(t *testing.T) {
	if _, err := Allocate(0, []float64{1}); err == nil {
		t.Fatal("expected error for zero total")
	}
	if _, err := Allocate(-5, []float64{1}); err == nil {
		t.Fatal("expected error for negative total")
	}
	if _, err := Allocate(10, nil); err == nil {
		t.Fatal("expected error for no weights")
	}
	if _, err := Allocate(10, []float64{0}); err == nil {
		t.Fatal("expected error for zero weight")
	}
	if _, err := Allocate(10, []float64{-1}); err == nil {
		t.Fatal("expected error for negative weight")
	}
}

func TestTwoTierSplitConservesTotal(t *testing.T) {
	tierOne, err := Allocate(10000, []float64{1, 1})
	if err != nil {
		t.Fatal(err)
	}
	group, err := Allocate(tierOne[1], []float64{3, 1})
	if err != nil {
		t.Fatal(err)
	}
	if sum(group) != tierOne[1] {
		t.Fatalf("group shares sum to %d, want %d", sum(group), tierOne[1])
	}
	if tierOne[0]+sum(group) != 10000 {
		t.Fatalf("two-tier split lost cents: %v", tierOne)
	}
}

func TestEvenWeights(t *testing.T) {
	weights := EvenWeights(4)
	if len(weights) != 4 {
		t.Fatalf("expected 4 weights, got %d", len(weights))
	}
	for _, w := range weights {
		if w != 1 {
			t.Fatalf("expected even weight 1, got %v", w)
		}
	}
}
