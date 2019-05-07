package cui

import (
	"image"
	"github.com/gizak/termui/v3"
	"strings"
)

type TextBox struct {
	termui.Block
	WrapText    bool
	TextStyle   termui.Style
	CursorStyle termui.Style
	ShowCursor  bool
	text        [][]termui.Cell
	cursorPoint image.Point
}
type TextBoxTheme struct {
	Text   termui.Style
	Cursor termui.Style
}

func NewTextBox() *TextBox {
	return &TextBox{
		Block:       *termui.NewBlock(),
		WrapText:    false,
		TextStyle:   termui.NewStyle(termui.ColorWhite),
		CursorStyle: termui.NewStyle(termui.ColorWhite, termui.ColorClear, termui.ModifierReverse),
		ShowCursor:  true,
		text:        [][]termui.Cell{[]termui.Cell{}},
		cursorPoint: image.Pt(1, 1),
	}
}

func (self *TextBox) Draw(buf *termui.Buffer) {
	self.Block.Draw(buf)

	yCoordinate := 0
	for _, line := range self.text {
		if self.WrapText {
			line = termui.WrapCells(line, uint(self.Inner.Dx()))
		}
		lines := termui.SplitCells(line, '\n')
		for _, line := range lines {
			for _, cx := range termui.BuildCellWithXArray(line) {
				x, cell := cx.X, cx.Cell
				buf.SetCell(cell, image.Pt(x, yCoordinate).Add(self.Inner.Min))
			}
			yCoordinate++
		}
		if yCoordinate > self.Inner.Max.Y {
			break
		}
	}

	if self.ShowCursor {
		point := self.cursorPoint.Add(self.Inner.Min).Sub(image.Pt(1, 1))
		cell := buf.GetCell(point)
		cell.Style = self.CursorStyle
		buf.SetCell(cell, point)
	}
}

func (self *TextBox) Backspace() {
	if self.cursorPoint == image.Pt(1, 1) {
		return
	}
	if self.cursorPoint.X == 1 {
		index := self.cursorPoint.Y - 1
		self.cursorPoint.X = len(self.text[index-1]) + 1
		self.text = append(
			self.text[:index-1],
			append(
				[][]termui.Cell{append(self.text[index-1], self.text[index]...)},
				self.text[index+1:len(self.text)]...,
			)...,
		)
		self.cursorPoint.Y--
	} else {
		index := self.cursorPoint.Y - 1
		self.text[index] = append(
			self.text[index][:self.cursorPoint.X-2],
			self.text[index][self.cursorPoint.X-1:]...,
		)
		self.cursorPoint.X--
	}
}

// InsertText inserts the given text at the cursor position.
func (self *TextBox) InsertText(input string) {
	cells := termui.ParseStyles(input, self.TextStyle)
	lines := termui.SplitCells(cells, '\n')
	index := self.cursorPoint.Y - 1
	cellsAfterCursor := self.text[index][self.cursorPoint.X-1:]
	self.text[index] = append(self.text[index][:self.cursorPoint.X-1], lines[0]...)
	for i, line := range lines[1:] {
		index := self.cursorPoint.Y + i
		self.text = append(self.text[:index], append([][]termui.Cell{line}, self.text[index:]...)...)
	}
	self.cursorPoint.Y += len(lines) - 1
	index = self.cursorPoint.Y - 1
	self.text[index] = append(self.text[index], cellsAfterCursor...)
	if len(lines) > 1 {
		self.cursorPoint.X = len(lines[len(lines)-1]) + 1
	} else {
		self.cursorPoint.X += len(lines[0])
	}
}

// ClearText clears the text and resets the cursor position.
func (self *TextBox) ClearText() {
	self.text = [][]termui.Cell{[]termui.Cell{}}
	self.cursorPoint = image.Pt(1, 1)
}

// SetText sets the text to the given text.
func (self *TextBox) SetText(input string) {
	self.ClearText()
	self.InsertText(input)
}

// GetText gets the text in string format along all its formatting tags
func (self *TextBox) GetText() string {
	return CellsToStyledString(JoinCells(self.text, '\n'), self.TextStyle)
}

// GetRawText gets the text in string format without any formatting tags
func (self *TextBox) GetRawText() string {
	return CellsToString(JoinCells(self.text, '\n'))
}

func (self *TextBox) MoveCursorLeft() {
	self.MoveCursor(self.cursorPoint.X-1, self.cursorPoint.Y)
}

func (self *TextBox) MoveCursorRight() {
	self.MoveCursor(self.cursorPoint.X+1, self.cursorPoint.Y)
}

func (self *TextBox) MoveCursorUp() {
	self.MoveCursor(self.cursorPoint.X, self.cursorPoint.Y-1)
}

func (self *TextBox) MoveCursorDown() {
	self.MoveCursor(self.cursorPoint.X, self.cursorPoint.Y+1)
}

func (self *TextBox) MoveCursor(x, y int) {
	self.cursorPoint.Y = termui.MinInt(termui.MaxInt(1, y), len(self.text))
	self.cursorPoint.X = termui.MinInt(termui.MaxInt(1, x), len(self.text[self.cursorPoint.Y-1])+1)
}

func CellsToStyledString(cells []termui.Cell, defaultStyle termui.Style) string {
	sb := strings.Builder{}
	runes := make([]rune, len(cells))
	currentStyle := cells[0].Style
	var j int

	for _, cell := range cells {
		if currentStyle != cell.Style {
			sb.WriteString(string(runes[:j]))

			currentStyle = cell.Style
			j = 0
		}

		runes[j] = cell.Rune
		j++
	}

	// Write the last characters left in runes slice
	sb.WriteString(string(runes[:j]))

	return sb.String()
}

func JoinCells(cells [][]termui.Cell, r rune) []termui.Cell {
	joinCells := make([]termui.Cell, 0)
	lb := termui.Cell{Rune: r, Style: termui.StyleClear}
	length := len(cells)

	for i, cell := range cells {
		if i < length-1 {
			cell = append(cell, lb)
		}
		joinCells = append(joinCells, cell...)
	}

	return joinCells
}

// CellsToString converts []Cell to a string without any formatting tags
func CellsToString(cells []termui.Cell) string {
	runes := make([]rune, len(cells))
	for i, cell := range cells {
		runes[i] = cell.Rune
	}
	return string(runes)
}

func ContainsString(a []string, s string) bool {
	for _, i := range a {
		if i == s {
			return true
		}
	}
	return false
}

var PRINTABLE_KEYS = append(
	strings.Split(
		"0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ,./<>?;:'\"[]\\{}|`~!@#$%^&*()-_=+",
		"",
	),
	"<Space>",
	"<Tab>",
	"<Enter>",
)