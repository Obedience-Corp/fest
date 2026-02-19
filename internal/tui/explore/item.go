package explore

import "time"

// ItemType identifies the level in the festival hierarchy.
type ItemType string

const (
	ItemStatus   ItemType = "status"
	ItemFestival ItemType = "festival"
	ItemPhase    ItemType = "phase"
	ItemSequence ItemType = "sequence"
	ItemTask     ItemType = "task"
)

// FestivalItem represents an entry at any level of the festival hierarchy.
type FestivalItem struct {
	Name      string
	Status    string
	Progress  float64
	CreatedAt time.Time
	Path      string
	Type      ItemType
	Count     int // Festival count for status items (-1 = loading)
}

// FilterValue implements the bubbles list.Item interface.
func (i FestivalItem) FilterValue() string {
	return i.Name
}
