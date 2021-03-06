package buffer

import (
	"bytes"
	"github.com/kisielk/vigo/utils"
	"unicode"
	"unicode/utf8"
)

// A Cursor represents a position within a buffer.
type Cursor struct {
	Line    *Line
	LineNum int
	Boffset int
}

// RuneUnder returns the rune under the current cursor and its width in bytes.
func (c *Cursor) RuneUnder() (rune, int) {
	return utf8.DecodeRune(c.Line.Data[c.Boffset:])
}

// RuneUnder returns the rune before the current cursor and its width in bytes.
func (c *Cursor) RuneBefore() (rune, int) {
	return utf8.DecodeLastRune(c.Line.Data[:c.Boffset])
}

// FirstLine returns true if the cursor is at the first line of the buffer.
func (c *Cursor) FirstLine() bool {
	return c.Line.Prev == nil
}

// LastLine returns true if the cursor is at the last line of the buffer.
func (c *Cursor) LastLine() bool {
	return c.Line.Next == nil
}

// EOL returns true if the cursor is at the end of the current line.
func (c *Cursor) EOL() bool {
	return c.Boffset == len(c.Line.Data)
}

// BOL returns true if the cursor is at the beginning of the current line.
func (c *Cursor) BOL() bool {
	return c.Boffset == 0
}

// Distance returns the distance between the cursor and another in bytes.
func (a Cursor) Distance(b Cursor) int {
	s := 1
	if b.LineNum < a.LineNum {
		a, b = b, a
		s = -1
	} else if a.LineNum == b.LineNum && b.Boffset < a.Boffset {
		a, b = b, a
		s = -1
	}

	n := 0
	for a.Line != b.Line {
		n += len(a.Line.Data) - a.Boffset + 1
		a.Line = a.Line.Next
		a.Boffset = 0
	}
	n += b.Boffset - a.Boffset
	return n * s
}

// VoffsetCoffset returns a visual and a character offset for a given cursor.
func (c *Cursor) VoffsetCoffset() (vo, co int) {
	data := c.Line.Data[:c.Boffset]
	for len(data) > 0 {
		r, rlen := utf8.DecodeRune(data)
		data = data[rlen:]
		co += 1
		vo += utils.RuneAdvanceLen(r, vo)
	}
	return
}

func (c *Cursor) ExtractBytes(n int) []byte {
	var buf bytes.Buffer
	offset := c.Boffset
	line := c.Line
	for n > 0 {
		switch {
		case offset < line.Len():
			nb := line.Len() - offset
			if n < nb {
				nb = n
			}
			buf.Write(line.Data[offset : offset+nb])
			n -= nb
			offset += nb
		case offset == line.Len():
			buf.WriteByte('\n')
			offset = 0
			line = line.Next
			n -= 1
		default:
			panic("unreachable")
		}
	}
	return buf.Bytes()
}

// NextRune moves cursor to the next rune. If wrap is true,
// wraps the cursor to the beginning of next line once the end
// of the current one is reached. Returns true if motion succeeded,
// false otherwise.
func (c *Cursor) NextRune(wrap bool) bool {
	if !c.EOL() {
		_, rlen := c.RuneUnder()
		c.Boffset += rlen
		return true
	} else if wrap && !c.LastLine() {
		c.Line = c.Line.Next
		c.LineNum++
		c.Boffset = 0
		return true
	}
	return false
}

// PrevRune moves cursor to the previous rune. If wrap is true,
// wraps the cursor to the end of next line once the beginning of
// the current one is reached. Returns true if motion succeeded,
// false otherwise.
func (c *Cursor) PrevRune(wrap bool) bool {
	if !c.BOL() {
		_, rlen := c.RuneBefore()
		c.Boffset -= rlen
		return true
	} else if wrap && !c.FirstLine() {
		c.Line = c.Line.Prev
		c.LineNum--
		c.Boffset = len(c.Line.Data)
		return true
	}
	return false
}

// MoveBOL moves the cursor to the beginning of the current line.
func (c *Cursor) MoveBOL() {
	c.Boffset = 0
}

// MoveEOL moves the cursor to the end of the current line.
func (c *Cursor) MoveEOL() {
	c.Boffset = len(c.Line.Data)
}

func (c *Cursor) WordUnderCursor() []byte {
	end, beg := *c, *c
	r, rlen := beg.RuneBefore()
	if r == utf8.RuneError {
		return nil
	}

	for utils.IsWord(r) && !beg.BOL() {
		beg.Boffset -= rlen
		r, rlen = beg.RuneBefore()
	}

	if beg.Boffset == end.Boffset {
		return nil
	}
	return c.Line.Data[beg.Boffset:end.Boffset]
}

// Move cursor forward until current rune satisfies condition f.
// Returns true if the move was successful, false if EOF reached.
func (c *Cursor) NextRuneFunc(f func(rune) bool) bool {
	for {
		if c.EOL() {
			if c.LastLine() {
				return false
			} else {
				c.Line = c.Line.Next
				c.LineNum++
				c.Boffset = 0
				continue
			}
		}
		r, rlen := c.RuneUnder()
		for !f(r) && !c.EOL() {
			c.Boffset += rlen
			r, rlen = c.RuneUnder()
		}
		if c.EOL() {
			continue
		}
		break
	}
	return true
}

// Move cursor forward to beginning of next word.
// Skips the rest of the current word, if any. Returns true if
// the move was successful, false if EOF reached.
func (c *Cursor) NextWord() bool {
	isNotSpace := func(r rune) bool {
		return !unicode.IsSpace(r)
	}
	r, _ := c.RuneUnder()
	if isNotSpace(r) {
		// Lowercase word motion differentiates words consisting of
		// (A-Z0-9_) and any other non-whitespace character. Skip until
		// we find either the other word type or whitespace.
		if utils.IsWord(r) {
			c.NextRuneFunc(func(r rune) bool {
				return !utils.IsWord(r) || unicode.IsSpace(r)
			})
		} else {
			c.NextRuneFunc(func(r rune) bool {
				return utils.IsWord(r) || unicode.IsSpace(r)
			})
		}
	}
	// Skip remaining whitespace until next word of any type.
	return c.NextRuneFunc(isNotSpace)
}

// EndWord moves cursor to the end of current word or seeks to the
// beginning of next word, if character under cursor is a whitespace.
func (c *Cursor) EndWord() bool {
	if !c.NextRune(true) {
		return false
	}

	// Skip spaces until beginning of next word
	r, _ := c.RuneUnder()
	if c.EOL() || unicode.IsSpace(r) {
		c.NextWord()
	}

	// Skip to after the word.
	r, _ = c.RuneUnder()
	var f func(r rune) bool
	if utils.IsWord(r) {
		f = func(r rune) bool {
			return !utils.IsWord(r) || unicode.IsSpace(r)
		}
	} else {
		f = func(r rune) bool {
			return utils.IsWord(r) || unicode.IsSpace(r)
		}
	}

	// This can go back to end of buffer but can be ignored,
	// since we're going to backtrack one character.
	c.NextRuneFunc(f)
	c.PrevRune(true)
	// Keep going back until BOF if we end up at EOL. This
	// can happen on empty lines.
	for c.EOL() && !(c.BOL() && c.FirstLine()) {
		c.PrevRune(true)
	}

	return true
}

// Move cursor backward until current rune satisfies condition f.
// Returns true if the move was successful, false if EOF reached.
func (c *Cursor) PrevRuneFunc(f func(rune) bool) bool {
	for {
		if c.BOL() {
			if c.FirstLine() {
				return false
			} else {
				c.Line = c.Line.Prev
				c.LineNum--
				c.Boffset = len(c.Line.Data)
				continue
			}
		}
		r, rlen := c.RuneBefore()
		for !f(r) && !c.BOL() {
			c.Boffset -= rlen
			r, rlen = c.RuneBefore()
		}
		break
	}
	return true
}

// Move cursor forward to beginning of the previous word.
// Skips the rest of the current word, if any, unless is located at its
// first character. Returns true if the move was successful, false if EOF reached.
func (c *Cursor) PrevWord() bool {
	isNotSpace := func(r rune) bool {
		return !unicode.IsSpace(r)
	}
	for {
		// Skip space until we find a word character.
		// Re-try if we reached beginning-of-line.
		if !c.PrevRuneFunc(isNotSpace) {
			return false
		}
		if !c.BOL() {
			break
		}
	}
	r, _ := c.RuneBefore()
	if isNotSpace(r) {
		// Lowercase word motion differentiates words consisting of
		// (A-Z0-9_) and any other non-whitespace character. Skip until
		// we find either the other word type or whitespace.
		if utils.IsWord(r) {
			c.PrevRuneFunc(func(r rune) bool {
				return !utils.IsWord(r) || unicode.IsSpace(r)
			})
		} else {
			c.PrevRuneFunc(func(r rune) bool {
				return utils.IsWord(r) || unicode.IsSpace(r)
			})
		}
	}
	return !c.BOL()
}

func (c *Cursor) OnInsertAdjust(a *Action) {
	if a.Cursor.LineNum > c.LineNum {
		return
	}
	if a.Cursor.LineNum < c.LineNum {
		// inserted something above the cursor, adjust it
		c.LineNum += len(a.Lines)
		return
	}

	// insertion on the cursor line
	if a.Cursor.Boffset <= c.Boffset {
		// insertion before or at the cursor, move cursor along with insertion
		if len(a.Lines) == 0 {
			// no lines were inserted, simply adjust the offset
			c.Boffset += len(a.Data)
		} else {
			// one or more lines were inserted, adjust cursor
			// respectively
			c.Line = a.LastLine()
			c.LineNum += len(a.Lines)
			c.Boffset = a.lastLineAffectionLen() +
				c.Boffset - a.Cursor.Boffset
		}
	}
}

func (c *Cursor) OnDeleteAdjust(a *Action) {
	if a.Cursor.LineNum > c.LineNum {
		return
	}
	if a.Cursor.LineNum < c.LineNum {
		// deletion above the cursor line, may touch the cursor location
		if len(a.Lines) == 0 {
			// no lines were deleted, no things to adjust
			return
		}

		first, last := a.DeletedLines()
		if first <= c.LineNum && c.LineNum <= last {
			// deleted the cursor line, see how much it affects it
			n := 0
			if last == c.LineNum {
				n = c.Boffset - a.lastLineAffectionLen()
				if n < 0 {
					n = 0
				}
			}
			*c = a.Cursor
			c.Boffset += n
		} else {
			// phew.. no worries
			c.LineNum -= len(a.Lines)
			return
		}
	}

	// the last case is deletion on the cursor line, see what was deleted
	if a.Cursor.Boffset >= c.Boffset {
		// deleted something after cursor, don't care
		return
	}

	n := c.Boffset - (a.Cursor.Boffset + a.firstLineAffectionLen())
	if n < 0 {
		n = 0
	}
	c.Boffset = a.Cursor.Boffset + n
}

func (c Cursor) searchForward(word []byte) (Cursor, bool) {
	for c.Line != nil {
		i := bytes.Index(c.Line.Data[c.Boffset:], word)
		if i != -1 {
			c.Boffset += i
			return c, true
		}

		c.Line = c.Line.Next
		c.LineNum++
		c.Boffset = 0
	}
	return c, false
}

func (c Cursor) searchBackward(word []byte) (Cursor, bool) {
	for {
		i := bytes.LastIndex(c.Line.Data[:c.Boffset], word)
		if i != -1 {
			c.Boffset = i
			return c, true
		}

		c.Line = c.Line.Prev
		if c.Line == nil {
			break
		}
		c.LineNum--
		c.Boffset = len(c.Line.Data)
	}
	return c, false
}

// SortCursors orders a pair of cursors, from closest to
// furthest from the beginning of the buffer.
func SortCursors(c1, c2 Cursor) (r1, r2 Cursor) {
	if c1.LineNum == c2.LineNum {
		if c1.Boffset > c2.Boffset {
			return c2, c1
		} else {
			return c1, c2
		}
	}

	if c1.LineNum > c2.LineNum {
		return c2, c1
	}
	return c1, c2
}
