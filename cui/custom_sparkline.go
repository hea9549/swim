/* 
 * Copyright 2019 hea9549
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package cui

import (
	"image"

	. "github.com/gizak/termui/v3"
)

// Sparkline is like: ▅▆▂▂▅▇▂▂▃▆▆▆▅▃. The data points should be non-negative integers.
type Sparkline struct {
	Data       []float64
	Title      string
	TitleStyle Style
	LineColor  Color
	MaxVal     float64
	MaxHeight  int // TODO
}

// SparklineGroup is a renderable widget which groups together the given sparklines.
type SparklineGroup struct {
	Block
	Sparklines []*Sparkline
}

// NewSparkline returns a unrenderable single sparkline that needs to be added to a SparklineGroup
func NewSparkline() *Sparkline {
	return &Sparkline{
		TitleStyle: Theme.Sparkline.Title,
		LineColor:  Theme.Sparkline.Line,
	}
}

func NewSparklineGroup(sls ...*Sparkline) *SparklineGroup {
	return &SparklineGroup{
		Block:      *NewBlock(),
		Sparklines: sls,
	}
}

func (self *SparklineGroup) Draw(buf *Buffer) {
	self.Block.Draw(buf)

	sparklineHeight := self.Inner.Dy() / len(self.Sparklines)

	for i, sl := range self.Sparklines {
		heightOffset := (sparklineHeight * (i + 1))
		barHeight := sparklineHeight
		if i == len(self.Sparklines)-1 {
			heightOffset = self.Inner.Dy()
			barHeight = self.Inner.Dy() - (sparklineHeight * i)
		}
		if sl.Title != "" {
			barHeight--
		}

		maxVal := sl.MaxVal
		if maxVal == 0 {
			maxVal, _ = GetMaxFloat64FromSlice(sl.Data)
		}

		// draw line
		for j := 0; j < len(sl.Data) && j < self.Inner.Dx(); j++ {
			data := sl.Data[j]
			height := int((data / maxVal) * float64(barHeight))
			runeHeight := data / maxVal
			var sparkChar rune
			switch {
			case runeHeight < float64(1)/float64(9):
				sparkChar = BARS[1]
				break
			case runeHeight < float64(2)/float64(9):
				sparkChar = BARS[2]
				break
			case runeHeight < float64(3)/float64(9):
				sparkChar = BARS[3]
				break
			case runeHeight < float64(4)/float64(9):
				sparkChar = BARS[4]
				break
			case runeHeight < float64(5)/float64(9):
				sparkChar = BARS[5]
				break
			case runeHeight < float64(6)/float64(9):
				sparkChar = BARS[6]
				break
			case runeHeight < float64(7)/float64(9):
				sparkChar = BARS[7]
				break
			case runeHeight < float64(8)/float64(9):
				sparkChar = BARS[8]
				break
			default:
				sparkChar = BARS[8]
				break
			}
			for k := 0; k < height; k++ {
				buf.SetCell(
					NewCell(sparkChar, NewStyle(sl.LineColor)),
					image.Pt(j+self.Inner.Min.X, self.Inner.Min.Y-1+heightOffset-k),
				)
			}
			if height == 0 {
				buf.SetCell(
					NewCell(sparkChar, NewStyle(sl.LineColor)),
					image.Pt(j+self.Inner.Min.X, self.Inner.Min.Y-1+heightOffset),
				)
			}
		}

		if sl.Title != "" {
			// draw title
			buf.SetString(
				TrimString(sl.Title, self.Inner.Dx()),
				sl.TitleStyle,
				image.Pt(self.Inner.Min.X, self.Inner.Min.Y-1+heightOffset-barHeight),
			)
		}
	}
}
