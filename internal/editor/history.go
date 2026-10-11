package editor

// history.go is the undo stack. Snapshots are whole-project deep copies
// — projects are small, and correctness beats cleverness here.

// History holds the undo/redo stacks around a live project.
type History struct {
	past   []*Project
	future []*Project
	limit  int
}

// NewHistory makes a history keeping at most limit past snapshots.
func NewHistory(limit int) *History {
	if limit <= 0 {
		limit = 100
	}
	return &History{limit: limit}
}

// Push records a pre-mutation snapshot; the future stack is dropped, the
// way any new edit cancels a redo.
func (h *History) Push(before *Project) {
	h.past = append(h.past, before)
	if len(h.past) > h.limit {
		h.past = h.past[len(h.past)-h.limit:]
	}
	h.future = nil
}

// CanUndo reports whether there is anything to undo.
func (h *History) CanUndo() bool { return len(h.past) > 0 }

// CanRedo reports whether there is anything to redo.
func (h *History) CanRedo() bool { return len(h.future) > 0 }

// Undo swaps the current project with the most recent snapshot and
// returns the restored one; nil when there is nothing to undo.
func (h *History) Undo(current *Project) *Project {
	if !h.CanUndo() {
		return nil
	}
	h.future = append(h.future, current)
	p := h.past[len(h.past)-1]
	h.past = h.past[:len(h.past)-1]
	return p
}

// Redo reapplies the most recently undone state; nil when there is
// nothing to redo.
func (h *History) Redo(current *Project) *Project {
	if !h.CanRedo() {
		return nil
	}
	h.past = append(h.past, current)
	p := h.future[len(h.future)-1]
	h.future = h.future[:len(h.future)-1]
	return p
}
