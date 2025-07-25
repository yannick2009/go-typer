// WARN:NOTES in this files are important!
// otehrwise it'll re-render exponentially on type!
package ui

import (
	"slices"
	"strings"
	"time"

	devlog "github.com/prime-run/go-typer/log"
)

type WordState int

const (
	Untyped WordState = iota
	Perfect
	Imperfect
	Error
)

type Word struct {
	target []rune
	typed  []rune
	state  WordState
	active bool
	cursor *Cursor
	cached string
	dirty  bool
}

func NewWord(target []rune) *Word {
	targetLen := len(target)
	targetCopy := make([]rune, targetLen)
	copy(targetCopy, target)

	return &Word{
		target: targetCopy,
		typed:  make([]rune, 0, targetLen),
		state:  Untyped,
		active: false,
		cursor: NewCursor(DefaultCursorType),
		dirty:  true, // NOTE:start with dirty cache
	}
}

func (w *Word) Type(r rune) {
	if w.IsSpace() {
		if r == ' ' {
			w.typed = []rune{' '}
			w.state = Perfect
		} else {
			w.typed = []rune{r}
			w.state = Error
		}
		w.dirty = true
		return
	}

	if len(w.typed) < len(w.target) {
		w.typed = append(w.typed, r)
	} else if len(w.typed) == len(w.target) {
		w.typed[len(w.typed)-1] = r
	}

	w.updateState()
	w.dirty = true
}

func (w *Word) Skip() {
	targetLen := len(w.target)
	typedLen := len(w.typed)

	if typedLen == 0 {
		// NOTE:optimize by pre-allocating the full array
		w.typed = make([]rune, targetLen)
		for i := 0; i < targetLen; i++ {
			w.typed[i] = '\x00'
		}
	} else if typedLen < targetLen {
		// NOTE:optimize by growing the slice once
		needed := targetLen - typedLen
		w.typed = append(w.typed, make([]rune, needed)...)
		for i := typedLen; i < targetLen; i++ {
			w.typed[i] = '\x00'
		}
	}

	w.state = Error
	w.dirty = true
}

func (w *Word) Backspace() bool {
	if len(w.typed) == 0 {
		return false
	}
	w.typed = w.typed[:len(w.typed)-1]
	w.updateState()
	w.dirty = true
	return true
}

func (w *Word) updateState() {
	if len(w.typed) == 0 {
		w.state = Untyped
		return
	}

	if w.IsSpace() {
		if len(w.typed) == 1 && w.typed[0] == ' ' {
			w.state = Perfect
		} else {
			w.state = Error
		}
		return
	}

	if slices.Contains(w.typed, '\x00') {
		w.state = Error
		return
	}

	minLen := min(len(w.typed), len(w.target))
	perfect := true
	for i := 0; i < minLen; i++ {
		if w.typed[i] != w.target[i] {
			perfect = false
			break
		}
	}

	if perfect && len(w.typed) == len(w.target) {
		w.state = Perfect
	} else if perfect && len(w.typed) < len(w.target) {
		w.state = Imperfect
	} else {
		w.state = Error
	}
}

func (w *Word) IsComplete() bool {
	complete := len(w.typed) >= len(w.target)
	devlog.Log("Word: IsComplete - Target: '%s' (%d), Typed: '%s' (%d), Complete: %v",
		string(w.target), len(w.target), string(w.typed), len(w.typed), complete)
	return complete
}

func (w *Word) HasStarted() bool {
	return len(w.typed) > 0
}

func (w *Word) IsSpace() bool {
	return len(w.target) == 1 && w.target[0] == ' '
}

func (w *Word) SetActive(active bool) {
	if w.active != active {
		w.active = active
		w.dirty = true
	}
}

func (w *Word) SetCursorType(cursorType CursorType) {
	w.cursor = NewCursor(cursorType)
	w.dirty = true
}

func (w *Word) Render(showCursor bool) string {
	//.
	//NOTE: If word is active, always render fresh
	// NOTE:If word is not active and not dirty, return cached result
	//.

	if !w.active && !w.dirty && w.cached != "" {
		return w.cached
	}

	startTime := time.Now()

	var result strings.Builder

	// NOTE:estimate buffer size to avoid reallocations
	// NOTE:allow extra space for style sequences

	result.Grow(max(len(w.target), len(w.typed)) * 3)
	if w.IsSpace() {
		if len(w.typed) == 0 {
			if showCursor && w.active {
				w.cached = w.cursor.Render(' ')
				return w.cached
			}
			w.cached = DimStyle.Render(" ")
			return w.cached
		} else if len(w.typed) == 1 && w.typed[0] == ' ' {
			w.cached = InputStyle.Render(" ")
			return w.cached
		} else {
			w.cached = ErrorStyle.Render(string(w.typed[0]))
			return w.cached
		}
	}

	targetLen := len(w.target)
	typedLen := len(w.typed)

	for i := 0; i < max(targetLen, typedLen); i++ {
		if showCursor && w.active && i == typedLen {
			if i < targetLen {
				result.WriteString(w.cursor.Render(w.target[i]))
			} else {
				result.WriteString(w.cursor.Render(' '))
			}
			continue
		}

		if i >= typedLen {
			result.WriteString(DimStyle.Render(string(w.target[i])))
			continue
		}

		if i >= targetLen {
			result.WriteString(ErrorStyle.Render(string(w.typed[i])))
			continue
		}

		if w.typed[i] == '\x00' {
			result.WriteString(DimStyle.Render(string(w.target[i])))
			continue
		}

		if w.typed[i] == w.target[i] {
			if w.state == Error {
				result.WriteString(PartialErrorStyle.Render(string(w.target[i])))
			} else {
				result.WriteString(InputStyle.Render(string(w.target[i])))
			}
		} else {
			result.WriteString(ErrorStyle.Render(string(w.typed[i])))
		}
	}

	rendered := result.String()

	if !w.active {
		w.cached = rendered
		w.dirty = false
	}

	if w.active {
		renderTime := time.Since(startTime)
		devlog.Log("Word: Active word render completed in %s, length: %d", renderTime, len(rendered))
	}

	return rendered
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
