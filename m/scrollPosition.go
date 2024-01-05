package m

import (
	"fmt"
	"math"

	"github.com/walles/moar/util"
)

// Please create using newScrollPosition(name)
type scrollPosition struct {
	internalDontTouch scrollPositionInternal
}

func newScrollPosition(name string) scrollPosition {
	if len(name) == 0 {
		panic("Non-empty name required")
	}
	return scrollPosition{
		internalDontTouch: scrollPositionInternal{
			name: name,
		},
	}
}

type scrollPositionInternal struct {
	// Line number in the input stream
	lineNumberOneBased int

	// Scroll this many screen lines before rendering. Can be negative.
	deltaScreenLines int

	name           string
	canonicalizing bool
	canonical      scrollPositionCanonical
}

// If any of these change, we have to recompute the scrollPositionInternal values
type scrollPositionCanonical struct {
	width           int  // From pager
	height          int  // From pager
	showLineNumbers bool // From pager
	showStatusBar   bool // From pager
	wrapLongLines   bool // From pager

	lineNumberOneBased int // From scrollPositionInternal
	deltaScreenLines   int // From scrollPositionInternal
}

func canonicalFromPager(pager *Pager) scrollPositionCanonical {
	width, height := pager.screen.Size()
	return scrollPositionCanonical{
		width:           width,
		height:          height,
		showLineNumbers: pager.ShowLineNumbers,
		showStatusBar:   pager.ShowStatusBar,
		wrapLongLines:   pager.WrapLongLines,

		lineNumberOneBased: pager.scrollPosition.internalDontTouch.lineNumberOneBased,
		deltaScreenLines:   pager.scrollPosition.internalDontTouch.deltaScreenLines,
	}
}

// Create a new position, scrolled towards the end of the file
func (sp scrollPosition) PreviousLine(scrollDistance int) scrollPosition {
	return scrollPosition{
		internalDontTouch: scrollPositionInternal{
			name:               sp.internalDontTouch.name,
			lineNumberOneBased: sp.internalDontTouch.lineNumberOneBased,
			deltaScreenLines:   sp.internalDontTouch.deltaScreenLines - scrollDistance,
		},
	}
}

// Create a new position, scrolled towards the end of the file
func (sp scrollPosition) NextLine(scrollDistance int) scrollPosition {
	return scrollPosition{
		internalDontTouch: scrollPositionInternal{
			name:               sp.internalDontTouch.name,
			lineNumberOneBased: sp.internalDontTouch.lineNumberOneBased,
			deltaScreenLines:   sp.internalDontTouch.deltaScreenLines + scrollDistance,
		},
	}
}

// Create a new position, scrolled to the given line number
//
//revive:disable-next-line:unexported-return
func NewScrollPositionFromLineNumberOneBased(lineNumberOneBased int, source string) scrollPosition {
	return scrollPosition{
		internalDontTouch: scrollPositionInternal{
			name:               source,
			lineNumberOneBased: lineNumberOneBased,
			deltaScreenLines:   0,
		},
	}
}

// Move towards the top until deltaScreenLines is not negative any more
func (si *scrollPositionInternal) handleNegativeDeltaScreenLines(pager *Pager) {
	for si.lineNumberOneBased > 1 && si.deltaScreenLines < 0 {
		// Render the previous line
		previousLine := pager.reader.GetLine(si.lineNumberOneBased - 1)
		previousSubLines, _ := pager.renderLine(previousLine, 0, *si)

		// Adjust lineNumberOneBased and deltaScreenLines to move up into the
		// previous screen line
		si.lineNumberOneBased--
		si.deltaScreenLines += len(previousSubLines)
	}

	if si.lineNumberOneBased <= 1 && si.deltaScreenLines <= 0 {
		// Don't go above the top line
		si.lineNumberOneBased = 1
		si.deltaScreenLines = 0
	}
}

// Move towards the bottom until deltaScreenLines is within range of the
// rendering of the current line.
//
// This method will not do any screen-height based clipping, so it could be that
// the position is too far down to display after this returns.
func (si *scrollPositionInternal) handlePositiveDeltaScreenLines(pager *Pager) {
	for {
		line := pager.reader.GetLine(si.lineNumberOneBased)
		if line == nil {
			// Out of bounds downwards, get the last line...
			si.lineNumberOneBased = pager.reader.GetLineCount()
			line = pager.reader.GetLine(si.lineNumberOneBased)
			if line == nil {
				panic(fmt.Errorf("Last line is nil"))
			}
			subLines, _ := pager.renderLine(line, 0, *si)

			// ... and go to the bottom of that.
			si.deltaScreenLines = len(subLines) - 1
			return
		}

		subLines, _ := pager.renderLine(line, 0, *si)
		if si.deltaScreenLines < len(subLines) {
			// Sublines are within bounds!
			return
		}

		si.lineNumberOneBased++
		si.deltaScreenLines -= len(subLines)
	}
}

// This method assumes si contains a canonical position
func (si *scrollPositionInternal) emptyBottomLinesCount(pager *Pager) int {
	unclaimedViewportLines := pager.visibleHeight()

	// Start counting where the current input line begins
	unclaimedViewportLines += si.deltaScreenLines

	lineNumberOneBased := si.lineNumberOneBased

	for {
		line := pager.reader.GetLine(lineNumberOneBased)
		if line == nil {
			// No more lines!
			break
		}

		subLines, _ := pager.renderLine(line, lineNumberOneBased, *si)
		unclaimedViewportLines -= len(subLines)
		if unclaimedViewportLines <= 0 {
			return 0
		}

		// Move to the next line
		lineNumberOneBased++
	}

	return unclaimedViewportLines
}

func (si *scrollPositionInternal) isCanonical(pager *Pager) bool {
	if si.canonical.lineNumberOneBased == 0 {
		// Awaiting initial lines from the reader
		return false
	}

	if si.canonical == canonicalFromPager(pager) {
		return true
	}

	return false
}

// Is the given position visible on screen?
func (sp scrollPosition) isVisible(pager *Pager) bool {
	if sp.internalDontTouch.deltaScreenLines < 0 {
		panic(fmt.Errorf("Negative incoming deltaScreenLines: %#v", sp.internalDontTouch))
	}

	if sp.internalDontTouch.lineNumberOneBased < pager.lineNumberOneBased() {
		// Line number too low, not visible
		return false
	}

	lastVisiblePosition := pager.getLastVisiblePosition()
	if sp.internalDontTouch.lineNumberOneBased > lastVisiblePosition.lineNumberOneBased(pager) {
		// Line number too high, not visible
		return false
	}

	// Line number is within range, now check the sub-line number

	if sp.internalDontTouch.deltaScreenLines > lastVisiblePosition.deltaScreenLines(pager) {
		// Sub-line-number too high, not visible
		return false
	}

	return true
}

// Only to be called from the scrollPosition getters!!
//
// Canonicalize the scroll position vs the given pager. A canonical position can
// just be displayed on screen, it has been clipped both towards the top and
// bottom of the screen, taking into account the screen height.
func (si *scrollPositionInternal) canonicalize(pager *Pager) {
	if si.isCanonical(pager) {
		return
	}

	if si.canonicalizing {
		panic(fmt.Errorf("Scroll position canonicalize() called recursively for %s", si.name))
	}
	si.canonicalizing = true

	defer func() {
		si.canonical = canonicalFromPager(pager)
		si.canonicalizing = false
	}()

	if pager.reader.GetLineCount() == 0 {
		si.lineNumberOneBased = 0
		si.deltaScreenLines = 0
		return
	}

	si.handleNegativeDeltaScreenLines(pager)
	si.handlePositiveDeltaScreenLines(pager)
	emptyBottomLinesCount := si.emptyBottomLinesCount(pager)
	if emptyBottomLinesCount > 0 {
		// First, adjust deltaScreenLines to get us to the top
		si.deltaScreenLines -= emptyBottomLinesCount

		// Then, actually go up that many lines
		si.handleNegativeDeltaScreenLines(pager)
	}
}

func scrollPositionFromLineNumber(name string, lineNumberOneBased int) *scrollPosition {
	return &scrollPosition{
		internalDontTouch: scrollPositionInternal{
			name:               name,
			lineNumberOneBased: lineNumberOneBased,
		},
	}
}

// Line number in the input stream, or 0 if nothing has been read
func (p *Pager) lineNumberOneBased() int {
	p.scrollPosition.internalDontTouch.canonicalize(p)
	return p.scrollPosition.internalDontTouch.lineNumberOneBased
}

// Line number in the input stream, or 0 if nothing has been read
func (sp *scrollPosition) lineNumberOneBased(pager *Pager) int {
	sp.internalDontTouch.canonicalize(pager)
	return sp.internalDontTouch.lineNumberOneBased
}

// Scroll this many screen lines before rendering
//
// Always >= 0.
func (p *Pager) deltaScreenLines() int {
	p.scrollPosition.internalDontTouch.canonicalize(p)
	return p.scrollPosition.internalDontTouch.deltaScreenLines
}

// Scroll this many screen lines before rendering
//
// Always >= 0.
func (sp *scrollPosition) deltaScreenLines(pager *Pager) int {
	sp.internalDontTouch.canonicalize(pager)
	return sp.internalDontTouch.deltaScreenLines
}

func (p *Pager) scrollToEnd() {
	inputLineCount := p.reader.GetLineCount()
	if inputLineCount == 0 {
		return
	}
	lastInputLineNumberOneBased := inputLineCount

	lastInputLine := p.reader.GetLine(lastInputLineNumberOneBased)

	p.scrollPosition.internalDontTouch.lineNumberOneBased = lastInputLineNumberOneBased

	// Scroll down enough. We know for sure the last line won't wrap into more
	// lines than the number of characters it contains.
	p.scrollPosition.internalDontTouch.deltaScreenLines = len(lastInputLine.raw)

	if p.TargetLineNumberOneBased == 0 {
		// Start following the end of the file
		//
		// Otherwise, if we're already aiming for some place, don't overwrite
		// that.
		p.TargetLineNumberOneBased = math.MaxInt
	}
}

// Can be either because Pager.scrollToEnd() was just called or because the user
// has pressed the down arrow enough times.
func (p *Pager) isScrolledToEnd() bool {
	inputLineCount := p.reader.GetLineCount()
	if inputLineCount == 0 {
		// No lines available, which means we can't scroll any further down
		return true
	}
	lastInputLineNumberOneBased := inputLineCount

	visibleLines, _, _ := p.renderLines()
	lastVisibleLine := visibleLines[len(visibleLines)-1]
	if lastVisibleLine.inputLineOneBased != lastInputLineNumberOneBased {
		// Last input line is not on the screen
		return false
	}

	// Last line is on screen, now we need to figure out whether we can see all
	// of it
	lastInputLine := p.reader.GetLine(lastInputLineNumberOneBased)
	lastInputLineRendered, _ := p.renderLine(lastInputLine, lastInputLineNumberOneBased, p.scrollPosition.internalDontTouch)
	lastRenderedSubLine := lastInputLineRendered[len(lastInputLineRendered)-1]

	// If the last visible subline is the same as the last possible subline then
	// we're at the bottom
	return lastVisibleLine.wrapIndex == lastRenderedSubLine.wrapIndex
}

// Returns nil if there are no lines
func (p *Pager) getLastVisiblePosition() *scrollPosition {
	renderedLines, _, _ := p.renderLines()
	if len(renderedLines) == 0 {
		return nil
	}

	lastRenderedLine := renderedLines[len(renderedLines)-1]
	return &scrollPosition{
		internalDontTouch: scrollPositionInternal{
			name:               "Last Visible Position",
			lineNumberOneBased: lastRenderedLine.inputLineOneBased,
			deltaScreenLines:   lastRenderedLine.wrapIndex,
		},
	}
}

func numberPrefixLength(pager *Pager, scrollPosition scrollPositionInternal) int {
	// This method used to live in screenLines.go, but I moved it here because
	// it touches scroll position internals.

	if !pager.ShowLineNumbers {
		return 0
	}

	maxPossibleLineNumber := pager.reader.GetLineCount()

	// This is an approximation assuming we don't do any wrapping. Finding the
	// real answer while wrapping requires rendering, which requires the real
	// answer and so on, so we do an approximation here to save us from
	// recursion.
	//
	// Let's improve on demand.
	maxVisibleLineNumber := (scrollPosition.lineNumberOneBased +
		scrollPosition.deltaScreenLines +
		pager.visibleHeight() - 1)
	if maxVisibleLineNumber > maxPossibleLineNumber {
		maxVisibleLineNumber = maxPossibleLineNumber
	}

	// Count the length of the last line number
	numberPrefixLength := len(util.FormatNumber(uint(maxVisibleLineNumber))) + 1
	if numberPrefixLength < 4 {
		// 4 = space for 3 digits followed by one whitespace
		//
		// https://github.com/walles/moar/issues/38
		numberPrefixLength = 4
	}

	return numberPrefixLength
}
