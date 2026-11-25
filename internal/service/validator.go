package service

import "fmt"

type GroupSelection struct {
	Name      string
	MemberIDs []int64
}

type ConflictError struct {
	MemberID     int64
	MemberName   string
	FirstSource  string
	SecondSource string
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf(
		"Conflict: %s is selected multiple times via %s and %s.",
		e.MemberName, e.FirstSource, e.SecondSource,
	)
}

// ValidateOverlap rejects selections where an individual member is also
// covered by a selected group, or two selected groups share a member.
func ValidateOverlap(memberIDs []int64, groups []GroupSelection, names map[int64]string) error {
	source := make(map[int64]string, len(memberIDs))

	for _, id := range memberIDs {
		if previous, exists := source[id]; exists {
			return &ConflictError{
				MemberID:     id,
				MemberName:   displayName(names, id),
				FirstSource:  previous,
				SecondSource: "Individual",
			}
		}
		source[id] = "Individual"
	}

	for _, group := range groups {
		sourceLabel := fmt.Sprintf("group %q", group.Name)
		for _, id := range group.MemberIDs {
			if previous, exists := source[id]; exists {
				return &ConflictError{
					MemberID:     id,
					MemberName:   displayName(names, id),
					FirstSource:  previous,
					SecondSource: sourceLabel,
				}
			}
			source[id] = sourceLabel
		}
	}

	return nil
}

func displayName(names map[int64]string, id int64) string {
	if name, ok := names[id]; ok && name != "" {
		return name
	}
	return fmt.Sprintf("member #%d", id)
}
