package cui

import (
	"image"
	"github.com/gizak/termui/v3"
)

type NodeStatParagraph struct {
	termui.Block
	Text      string
	TextStyle termui.Style
	WrapText  bool
}

func NewNodeStatParagraph() *NodeStatParagraph {
	return &NodeStatParagraph{
		Block:     *termui.NewBlock(),
		TextStyle: termui.Theme.Paragraph.Text,
		WrapText:  true,
	}
}

func (self *NodeStatParagraph) Draw(buf *termui.Buffer) {
	self.Block.Draw(buf)

	cells := termui.ParseStyles(self.Text, self.TextStyle)
	if self.WrapText {
		cells = termui.WrapCells(cells, uint(self.Inner.Dx()))
	}

	rows := termui.SplitCells(cells, '\n')

	for y, row := range rows {
		if y+self.Inner.Min.Y >= self.Inner.Max.Y {
			break
		}
		row = termui.TrimCells(row, self.Inner.Dx())
		for _, cx := range termui.BuildCellWithXArray(row) {
			x, cell := cx.X, cx.Cell
			switch string(cell.Rune) {
			case "R":
				cell.Style=termui.NewStyle(termui.ColorRed, termui.ColorClear,termui.ModifierReverse)
				cell.Rune=' '
			case "G":
				cell.Style=termui.NewStyle(termui.ColorGreen, termui.ColorClear,termui.ModifierReverse)
				cell.Rune=' '
			case "B":
				cell.Style=termui.NewStyle(termui.ColorBlue, termui.ColorClear,termui.ModifierReverse)
				cell.Rune=' '
			case "Y":
				cell.Style=termui.NewStyle(termui.ColorYellow, termui.ColorClear,termui.ModifierReverse)
				cell.Rune=' '
			case "M":
				cell.Style=termui.NewStyle(termui.ColorMagenta, termui.ColorClear,termui.ModifierReverse)
				cell.Rune=' '
			case "C":
				cell.Style=termui.NewStyle(termui.ColorCyan, termui.ColorClear,termui.ModifierReverse)
				cell.Rune=' '
			case "W":
				cell.Style=termui.NewStyle(termui.ColorWhite, termui.ColorClear,termui.ModifierReverse)
				cell.Rune=' '
			}
			buf.SetCell(cell, image.Pt(x, y).Add(self.Inner.Min))
		}
	}
}

