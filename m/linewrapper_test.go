package m

import (
	"reflect"
	"testing"

	"github.com/walles/moar/readers"
	"github.com/walles/moar/twin"
)

func tokenize(input string) []twin.Cell {
	line := readers.NewLine(input)
	return line.HighlightedTokens("", nil, nil).Cells
}

func rowsToString(cellLines [][]twin.Cell) string {
	returnMe := ""
	for _, cellLine := range cellLines {
		lineString := ""
		for _, cell := range cellLine {
			lineString += string(cell.Rune)
		}

		if len(returnMe) > 0 {
			returnMe += "\n"
		}
		returnMe += "<" + lineString + ">"
	}

	return returnMe
}

func assertWrap(t *testing.T, input string, width int, wrappedLines ...string) {
	toWrap := tokenize(input)
	actual := wrapLine(width, toWrap)

	expected := [][]twin.Cell{}
	for _, wrappedLine := range wrappedLines {
		expected = append(expected, tokenize(wrappedLine))
	}

	if reflect.DeepEqual(actual, expected) {
		return
	}

	t.Errorf("When wrapping <%s> at width %d:\n--Expected--\n%s\n\n--Actual--\n%s",
		input, width, rowsToString(expected), rowsToString(actual))
}

func TestEnoughRoomNoWrapping(t *testing.T) {
	assertWrap(t, "This is a test", 20, "This is a test")
}

func TestWrapBlank(t *testing.T) {
	assertWrap(t, "    ", 4, "")
	assertWrap(t, "    ", 2, "")

	assertWrap(t, "", 20, "")
}

func TestWordLongerThanLine(t *testing.T) {
	assertWrap(t, "intermediary", 6, "interm", "ediary")
}

func TestLeadingSpaceNoWrap(t *testing.T) {
	assertWrap(t, " abc", 20, " abc")
}

func TestLeadingSpaceWithWrap(t *testing.T) {
	assertWrap(t, " abc", 2, " a", "bc")
}

func TestLeadingWrappedSpace(t *testing.T) {
	assertWrap(t, "ab cd", 2, "ab", "cd")
}

func TestWordWrap(t *testing.T) {
	assertWrap(t, "abc 123", 8, "abc 123")
	assertWrap(t, "abc 123", 7, "abc 123")
	assertWrap(t, "abc 123", 6, "abc", "123")
	assertWrap(t, "abc 123", 5, "abc", "123")
	assertWrap(t, "abc 123", 4, "abc", "123")
	assertWrap(t, "abc 123", 3, "abc", "123")
	assertWrap(t, "abc 123", 2, "ab", "c", "12", "3")
}

func TestWordWrapUrl(t *testing.T) {
	assertWrap(t, "http://apa/bepa/", 17, "http://apa/bepa/")
	assertWrap(t, "http://apa/bepa/", 16, "http://apa/bepa/")
	assertWrap(t, "http://apa/bepa/", 15, "http://apa/", "bepa/")
	assertWrap(t, "http://apa/bepa/", 14, "http://apa/", "bepa/")
	assertWrap(t, "http://apa/bepa/", 13, "http://apa/", "bepa/")
	assertWrap(t, "http://apa/bepa/", 12, "http://apa/", "bepa/")
	assertWrap(t, "http://apa/bepa/", 11, "http://apa/", "bepa/")
	assertWrap(t, "http://apa/bepa/", 10, "http://apa", "/bepa/")
	assertWrap(t, "http://apa/bepa/", 9, "http://ap", "a/bepa/")
	assertWrap(t, "http://apa/bepa/", 8, "http://a", "pa/bepa/")
	assertWrap(t, "http://apa/bepa/", 7, "http://", "apa/", "bepa/")
	assertWrap(t, "http://apa/bepa/", 6, "http:/", "/apa/", "bepa/")
	assertWrap(t, "http://apa/bepa/", 5, "http:", "//apa", "/bepa", "/")
	assertWrap(t, "http://apa/bepa/", 4, "http", "://a", "pa/", "bepa", "/")
	assertWrap(t, "http://apa/bepa/", 3, "htt", "p:/", "/ap", "a/", "bep", "a/")
}

func TestWordWrapMarkdownLink(t *testing.T) {
	assertWrap(t, "[something](http://apa/bepa)", 13, "[something]", "(http://apa/", "bepa)")
	assertWrap(t, "[something](http://apa/bepa)", 12, "[something]", "(http://apa/", "bepa)")
	assertWrap(t, "[something](http://apa/bepa)", 11, "[something]", "(http://apa", "/bepa)")

	// This doesn't look great, room for tuning!
	assertWrap(t, "[something](http://apa/bepa)", 10, "[something", "]", "(http://ap", "a/bepa)")
}
