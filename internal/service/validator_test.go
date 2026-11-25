package service

import (
	"errors"
	"testing"
)

func names() map[int64]string {
	return map[int64]string{
		1: "Alex",
		2: "Sam",
		3: "John",
		4: "Jane",
	}
}

func TestValidateOverlapAllowsDistinctSelections(t *testing.T) {
	err := ValidateOverlap(
		[]int64{1},
		[]GroupSelection{{Name: "John & Jane", MemberIDs: []int64{3, 4}}},
		names(),
	)
	if err != nil {
		t.Fatalf("expected no conflict, got %v", err)
	}
}

func TestValidateOverlapRejectsDuplicateIndividual(t *testing.T) {
	err := ValidateOverlap([]int64{1, 1}, nil, names())
	if err == nil {
		t.Fatal("expected conflict for duplicate individual")
	}
	var conflict *ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("expected ConflictError, got %T", err)
	}
	if conflict.MemberName != "Alex" {
		t.Fatalf("expected member Alex, got %q", conflict.MemberName)
	}
}

func TestValidateOverlapRejectsIndividualInsideGroup(t *testing.T) {
	err := ValidateOverlap(
		[]int64{3},
		[]GroupSelection{{Name: "John & Jane", MemberIDs: []int64{3, 4}}},
		names(),
	)
	if err == nil {
		t.Fatal("expected conflict for individual inside group")
	}
}

func TestValidateOverlapRejectsIndividualSelectedAfterGroup(t *testing.T) {
	err := ValidateOverlap(
		[]int64{4},
		[]GroupSelection{{Name: "John & Jane", MemberIDs: []int64{3, 4}}},
		names(),
	)
	if err == nil {
		t.Fatal("expected conflict when group covers a selected individual")
	}
}

func TestValidateOverlapRejectsSharedGroupMember(t *testing.T) {
	err := ValidateOverlap(
		nil,
		[]GroupSelection{
			{Name: "John & Jane", MemberIDs: []int64{3, 4}},
			{Name: "Sam & Jane", MemberIDs: []int64{2, 4}},
		},
		names(),
	)
	if err == nil {
		t.Fatal("expected conflict for shared group member")
	}
	var conflict *ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("expected ConflictError, got %T", err)
	}
	if conflict.MemberName != "Jane" {
		t.Fatalf("expected member Jane, got %q", conflict.MemberName)
	}
}

func TestConflictErrorMentionsBothSources(t *testing.T) {
	err := ValidateOverlap(
		[]int64{3},
		[]GroupSelection{{Name: "John & Jane", MemberIDs: []int64{3, 4}}},
		names(),
	)
	var conflict *ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("expected ConflictError, got %T", err)
	}
	want := `Conflict: John is selected multiple times via Individual and group "John & Jane".`
	if conflict.Error() != want {
		t.Fatalf("got %q, want %q", conflict.Error(), want)
	}
}
