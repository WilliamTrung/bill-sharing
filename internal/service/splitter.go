package service

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"sort"
)

const (
	weightScale   = 1_000_000
	maxWeight     = 1_000_000
	maxTotalCents = 100_000_000_000
)

var ErrInvalidTotal = errors.New("total must be a positive amount")

// Allocate splits totalCents across the given weights using the largest
// remainder method. The returned shares always sum exactly to totalCents.
func Allocate(totalCents int64, weights []float64) ([]int64, error) {
	if totalCents <= 0 {
		return nil, ErrInvalidTotal
	}
	if totalCents > maxTotalCents {
		return nil, errors.New("total amount exceeds the supported maximum")
	}
	if len(weights) == 0 {
		return nil, errors.New("at least one participant is required")
	}

	scaled := make([]int64, len(weights))
	var weightSum int64
	for i, w := range weights {
		if math.IsNaN(w) || math.IsInf(w, 0) || w <= 0 || w > maxWeight {
			return nil, fmt.Errorf("participant %d has an invalid weight", i+1)
		}
		s := int64(math.Round(w * weightScale))
		if s <= 0 {
			s = 1
		}
		scaled[i] = s
		weightSum += s
	}

	total := big.NewInt(totalCents)
	denom := big.NewInt(weightSum)
	shares := make([]int64, len(scaled))
	remainders := make([]*big.Int, len(scaled))
	var allocated int64

	for i, s := range scaled {
		numerator := new(big.Int).Mul(total, big.NewInt(s))
		quotient, remainder := new(big.Int), new(big.Int)
		quotient.QuoRem(numerator, denom, remainder)
		shares[i] = quotient.Int64()
		remainders[i] = remainder
		allocated += shares[i]
	}

	leftover := int(totalCents - allocated)
	if leftover < 0 || leftover >= len(shares) {
		return nil, errors.New("internal allocation error")
	}

	order := make([]int, len(shares))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool {
		return remainders[order[a]].Cmp(remainders[order[b]]) > 0
	})
	for i := 0; i < leftover; i++ {
		shares[order[i]]++
	}

	return shares, nil
}

func EvenWeights(n int) []float64 {
	weights := make([]float64, n)
	for i := range weights {
		weights[i] = 1
	}
	return weights
}
