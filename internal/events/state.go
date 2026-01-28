package events

// MaterializeState builds current festival state from a sequence of events.
// Events are processed in order to derive the final state of each festival.
func MaterializeState(eventsList []FestivalEvent) map[string]*Festival {
	state := make(map[string]*Festival)

	for _, e := range eventsList {
		switch e.Event {
		case EventCreate:
			state[e.ID] = &Festival{
				ID:        e.ID,
				Name:      e.Name,
				Status:    e.Status,
				CreatedAt: e.Timestamp,
			}

		case EventMove:
			if f, ok := state[e.ID]; ok {
				f.Status = e.To
			}

		case EventArchive:
			if f, ok := state[e.ID]; ok {
				f.Status = "dungeon"
			}

		case EventDelete:
			delete(state, e.ID)
		}
	}

	return state
}

// FindByID returns a festival by ID from materialized state.
func FindByID(state map[string]*Festival, id string) (*Festival, bool) {
	f, ok := state[id]
	return f, ok
}

// FilterByStatus returns all festivals with the given status.
func FilterByStatus(state map[string]*Festival, status string) []*Festival {
	var result []*Festival
	for _, f := range state {
		if f.Status == status {
			result = append(result, f)
		}
	}
	return result
}
