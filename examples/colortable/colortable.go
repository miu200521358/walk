// Copyright 2025 The Walk Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"sort"
	"strings"

	. "github.com/miu200521358/walk/pkg/declarative"
	"github.com/miu200521358/walk/pkg/walk"
)

type ColorInfo struct {
	Name      string
	Color     walk.Color
	R, G, B   byte
	Hex       string
	Luminance float64
}

type ColorModel struct {
	walk.TableModelBase
	walk.SorterBase
	sortColumn int
	sortOrder  walk.SortOrder
	items      []*ColorInfo
}

func NewColorModel() *ColorModel {
	m := new(ColorModel)
	m.loadColors()
	return m
}

func (m *ColorModel) loadColors() {
	var colors []*ColorInfo

	// 全てのColor定数を手動で追加（リフレクションでは難しいため）
	colorMap := map[string]walk.Color{
		"AliceBlue":            walk.ColorAliceBlue,
		"AntiqueWhite":         walk.ColorAntiqueWhite,
		"Aqua":                 walk.ColorAqua,
		"Aquamarine":           walk.ColorAquamarine,
		"Azure":                walk.ColorAzure,
		"Beige":                walk.ColorBeige,
		"Bisque":               walk.ColorBisque,
		"Black":                walk.ColorBlack,
		"BlanchedAlmond":       walk.ColorBlanchedAlmond,
		"Blue":                 walk.ColorBlue,
		"BlueViolet":           walk.ColorBlueViolet,
		"Brown":                walk.ColorBrown,
		"BurlyWood":            walk.ColorBurlyWood,
		"CadetBlue":            walk.ColorCadetBlue,
		"Chartreuse":           walk.ColorChartreuse,
		"Chocolate":            walk.ColorChocolate,
		"Coral":                walk.ColorCoral,
		"CornflowerBlue":       walk.ColorCornflowerBlue,
		"Cornsilk":             walk.ColorCornsilk,
		"Crimson":              walk.ColorCrimson,
		"Cyan":                 walk.ColorCyan,
		"DarkBlue":             walk.ColorDarkBlue,
		"DarkCyan":             walk.ColorDarkCyan,
		"DarkGoldenrod":        walk.ColorDarkGoldenrod,
		"DarkGray":             walk.ColorDarkGray,
		"DarkGreen":            walk.ColorDarkGreen,
		"DarkKhaki":            walk.ColorDarkKhaki,
		"DarkMagenta":          walk.ColorDarkMagenta,
		"DarkOliveGreen":       walk.ColorDarkOliveGreen,
		"DarkOrange":           walk.ColorDarkOrange,
		"DarkOrchid":           walk.ColorDarkOrchid,
		"DarkRed":              walk.ColorDarkRed,
		"DarkSalmon":           walk.ColorDarkSalmon,
		"DarkSeaGreen":         walk.ColorDarkSeaGreen,
		"DarkSlateBlue":        walk.ColorDarkSlateBlue,
		"DarkSlateGray":        walk.ColorDarkSlateGray,
		"DarkTurquoise":        walk.ColorDarkTurquoise,
		"DarkViolet":           walk.ColorDarkViolet,
		"DeepPink":             walk.ColorDeepPink,
		"DeepSkyBlue":          walk.ColorDeepSkyBlue,
		"DimGray":              walk.ColorDimGray,
		"DodgerBlue":           walk.ColorDodgerBlue,
		"Firebrick":            walk.ColorFirebrick,
		"FloralWhite":          walk.ColorFloralWhite,
		"ForestGreen":          walk.ColorForestGreen,
		"Fuchsia":              walk.ColorFuchsia,
		"Gainsboro":            walk.ColorGainsboro,
		"GhostWhite":           walk.ColorGhostWhite,
		"Gold":                 walk.ColorGold,
		"Goldenrod":            walk.ColorGoldenrod,
		"Gray":                 walk.ColorGray,
		"Green":                walk.ColorGreen,
		"GreenYellow":          walk.ColorGreenYellow,
		"Honeydew":             walk.ColorHoneydew,
		"HotPink":              walk.ColorHotPink,
		"IndianRed":            walk.ColorIndianRed,
		"Indigo":               walk.ColorIndigo,
		"Ivory":                walk.ColorIvory,
		"Khaki":                walk.ColorKhaki,
		"Lavender":             walk.ColorLavender,
		"LavenderBlush":        walk.ColorLavenderBlush,
		"LawnGreen":            walk.ColorLawnGreen,
		"LemonChiffon":         walk.ColorLemonChiffon,
		"LightBlue":            walk.ColorLightBlue,
		"LightCoral":           walk.ColorLightCoral,
		"LightCyan":            walk.ColorLightCyan,
		"LightGoldenrodYellow": walk.ColorLightGoldenrodYellow,
		"LightGray":            walk.ColorLightGray,
		"LightGreen":           walk.ColorLightGreen,
		"LightPink":            walk.ColorLightPink,
		"LightSalmon":          walk.ColorLightSalmon,
		"LightSeaGreen":        walk.ColorLightSeaGreen,
		"LightSkyBlue":         walk.ColorLightSkyBlue,
		"LightSlateGray":       walk.ColorLightSlateGray,
		"LightSteelBlue":       walk.ColorLightSteelBlue,
		"LightYellow":          walk.ColorLightYellow,
		"Lime":                 walk.ColorLime,
		"LimeGreen":            walk.ColorLimeGreen,
		"Linen":                walk.ColorLinen,
		"Magenta":              walk.ColorMagenta,
		"Maroon":               walk.ColorMaroon,
		"MediumAquamarine":     walk.ColorMediumAquamarine,
		"MediumBlue":           walk.ColorMediumBlue,
		"MediumOrchid":         walk.ColorMediumOrchid,
		"MediumPurple":         walk.ColorMediumPurple,
		"MediumSeaGreen":       walk.ColorMediumSeaGreen,
		"MediumSlateBlue":      walk.ColorMediumSlateBlue,
		"MediumSpringGreen":    walk.ColorMediumSpringGreen,
		"MediumTurquoise":      walk.ColorMediumTurquoise,
		"MediumVioletRed":      walk.ColorMediumVioletRed,
		"MidnightBlue":         walk.ColorMidnightBlue,
		"MintCream":            walk.ColorMintCream,
		"MistyRose":            walk.ColorMistyRose,
		"Moccasin":             walk.ColorMoccasin,
		"NavajoWhite":          walk.ColorNavajoWhite,
		"Navy":                 walk.ColorNavy,
		"OldLace":              walk.ColorOldLace,
		"Olive":                walk.ColorOlive,
		"OliveDrab":            walk.ColorOliveDrab,
		"Orange":               walk.ColorOrange,
		"OrangeRed":            walk.ColorOrangeRed,
		"Orchid":               walk.ColorOrchid,
		"PaleGoldenrod":        walk.ColorPaleGoldenrod,
		"PaleGreen":            walk.ColorPaleGreen,
		"PaleTurquoise":        walk.ColorPaleTurquoise,
		"PaleVioletRed":        walk.ColorPaleVioletRed,
		"PapayaWhip":           walk.ColorPapayaWhip,
		"PeachPuff":            walk.ColorPeachPuff,
		"Peru":                 walk.ColorPeru,
		"Pink":                 walk.ColorPink,
		"Plum":                 walk.ColorPlum,
		"PowderBlue":           walk.ColorPowderBlue,
		"Purple":               walk.ColorPurple,
		"Red":                  walk.ColorRed,
		"RosyBrown":            walk.ColorRosyBrown,
		"RoyalBlue":            walk.ColorRoyalBlue,
		"SaddleBrown":          walk.ColorSaddleBrown,
		"Salmon":               walk.ColorSalmon,
		"SandyBrown":           walk.ColorSandyBrown,
		"SeaGreen":             walk.ColorSeaGreen,
		"SeaShell":             walk.ColorSeaShell,
		"Sienna":               walk.ColorSienna,
		"Silver":               walk.ColorSilver,
		"SkyBlue":              walk.ColorSkyBlue,
		"SlateBlue":            walk.ColorSlateBlue,
		"SlateGray":            walk.ColorSlateGray,
		"Snow":                 walk.ColorSnow,
		"SpringGreen":          walk.ColorSpringGreen,
		"SteelBlue":            walk.ColorSteelBlue,
		"Tan":                  walk.ColorTan,
		"Teal":                 walk.ColorTeal,
		"Thistle":              walk.ColorThistle,
		"Tomato":               walk.ColorTomato,
		"Turquoise":            walk.ColorTurquoise,
		"Violet":               walk.ColorViolet,
		"Wheat":                walk.ColorWheat,
		"White":                walk.ColorWhite,
		"WhiteSmoke":           walk.ColorWhiteSmoke,
		"Yellow":               walk.ColorYellow,
		"YellowGreen":          walk.ColorYellowGreen,
	}

	for name, color := range colorMap {
		r, g, b := color.R(), color.G(), color.B()

		// 輝度計算（相対輝度）
		luminance := 0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)

		colors = append(colors, &ColorInfo{
			Name:      name,
			Color:     color,
			R:         r,
			G:         g,
			B:         b,
			Hex:       fmt.Sprintf("#%02X%02X%02X", r, g, b),
			Luminance: luminance,
		})
	}

	m.items = colors
	m.PublishRowsReset()
}

func (m *ColorModel) RowCount() int {
	return len(m.items)
}

func (m *ColorModel) Value(row, col int) interface{} {
	item := m.items[row]

	switch col {
	case 0:
		return item.Name
	case 1:
		return fmt.Sprintf("(%d, %d, %d)", item.R, item.G, item.B)
	case 2:
		return item.Hex
	case 3:
		return fmt.Sprintf("%.0f", item.Luminance)
	}

	panic("unexpected col")
}

func (m *ColorModel) Sort(col int, order walk.SortOrder) error {
	m.sortColumn, m.sortOrder = col, order

	sort.SliceStable(m.items, func(i, j int) bool {
		a, b := m.items[i], m.items[j]

		c := func(ls bool) bool {
			if m.sortOrder == walk.SortAscending {
				return ls
			}
			return !ls
		}

		switch m.sortColumn {
		case 0:
			return c(strings.ToLower(a.Name) < strings.ToLower(b.Name))
		case 1:
			return c(a.R < b.R || (a.R == b.R && a.G < b.G) || (a.R == b.R && a.G == b.G && a.B < b.B))
		case 2:
			return c(a.Hex < b.Hex)
		case 3:
			return c(a.Luminance < b.Luminance)
		}

		panic("unreachable")
	})

	return m.SorterBase.Sort(col, order)
}

func main() {
	model := NewColorModel()

	var tv *walk.TableView

	MainWindow{
		Title:  "Walk Color Constants Table",
		Size:   Size{800, 600},
		Layout: VBox{MarginsZero: true},
		Children: []Widget{
			Label{
				Text: "Walkライブラリで定義されているColor定数一覧",
				Font: Font{PointSize: 12, Bold: true},
			},
			HSpacer{Size: 10},
			TableView{
				AssignTo:         &tv,
				AlternatingRowBG: false, // 背景色を設定するため無効化
				ColumnsOrderable: true,
				Columns: []TableViewColumn{
					{Title: "色名", Width: 200},
					{Title: "RGB値", Width: 120},
					{Title: "16進数", Width: 100},
					{Title: "輝度", Width: 80},
				},
				StyleCell: func(style *walk.CellStyle) {
					item := model.items[style.Row()]

					// 背景色を実際の色に設定
					style.BackgroundColor = item.Color

					// 輝度に応じてテキスト色を調整
					if item.Luminance > 128 {
						style.TextColor = walk.RGB(0, 0, 0) // 明るい背景には黒文字
					} else {
						style.TextColor = walk.RGB(255, 255, 255) // 暗い背景には白文字
					}
				},
				Model: model,
			},
		},
	}.Run()
}
