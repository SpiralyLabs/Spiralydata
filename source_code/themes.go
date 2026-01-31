package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// ThemeType représente le type de thème
type ThemeType int

const (
	ThemeDark ThemeType = iota
	ThemeLight
)

// AppTheme est le thème personnalisé de l'application
type AppTheme struct {
	themeType ThemeType
}

// NewAppTheme crée un nouveau thème
func NewAppTheme(t ThemeType) *AppTheme {
	return &AppTheme{themeType: t}
}

// Couleurs du thème sombre
var darkColors = map[fyne.ThemeColorName]color.Color{
	theme.ColorNameBackground:      color.RGBA{R: 30, G: 33, B: 41, A: 255},
	theme.ColorNameButton:          color.RGBA{R: 52, G: 58, B: 70, A: 255},
	theme.ColorNameForeground:      color.RGBA{R: 255, G: 255, B: 255, A: 255},
	theme.ColorNameSuccess:         color.RGBA{R: 46, G: 204, B: 113, A: 255},
	theme.ColorNameError:           color.RGBA{R: 231, G: 76, B: 60, A: 255},
	theme.ColorNameWarning:         color.RGBA{R: 241, G: 196, B: 15, A: 255},
	theme.ColorNameDisabled:        color.RGBA{R: 150, G: 150, B: 150, A: 255},
	theme.ColorNameInputBackground: color.RGBA{R: 40, G: 44, B: 52, A: 255},
	theme.ColorNamePlaceHolder:     color.RGBA{R: 128, G: 128, B: 140, A: 255},
	theme.ColorNamePrimary:         color.RGBA{R: 79, G: 134, B: 247, A: 255},
	theme.ColorNameHover:           color.RGBA{R: 60, G: 65, B: 80, A: 255},
	theme.ColorNameFocus:           color.RGBA{R: 79, G: 134, B: 247, A: 255},
	theme.ColorNameSelection:       color.RGBA{R: 79, G: 134, B: 247, A: 100},
	theme.ColorNameShadow:          color.RGBA{R: 0, G: 0, B: 0, A: 100},
	theme.ColorNameScrollBar:       color.RGBA{R: 80, G: 85, B: 100, A: 255},
	theme.ColorNameSeparator:       color.RGBA{R: 60, G: 65, B: 80, A: 255},
}

// Couleurs du thème clair
var lightColors = map[fyne.ThemeColorName]color.Color{
	theme.ColorNameBackground:      color.RGBA{R: 250, G: 250, B: 252, A: 255},
	theme.ColorNameButton:          color.RGBA{R: 230, G: 232, B: 238, A: 255},
	theme.ColorNameForeground:      color.RGBA{R: 30, G: 33, B: 41, A: 255},
	theme.ColorNameSuccess:         color.RGBA{R: 39, G: 174, B: 96, A: 255},
	theme.ColorNameError:           color.RGBA{R: 192, G: 57, B: 43, A: 255},
	theme.ColorNameWarning:         color.RGBA{R: 211, G: 166, B: 37, A: 255},
	theme.ColorNameDisabled:        color.RGBA{R: 60, G: 60, B: 70, A: 255}, // Plus foncé pour lisibilité
	theme.ColorNameInputBackground: color.RGBA{R: 255, G: 255, B: 255, A: 255},
	theme.ColorNamePlaceHolder:     color.RGBA{R: 120, G: 120, B: 130, A: 255},
	theme.ColorNamePrimary:         color.RGBA{R: 59, G: 114, B: 227, A: 255},
	theme.ColorNameHover:           color.RGBA{R: 220, G: 222, B: 228, A: 255},
	theme.ColorNameFocus:           color.RGBA{R: 59, G: 114, B: 227, A: 255},
	theme.ColorNameSelection:       color.RGBA{R: 59, G: 114, B: 227, A: 80},
	theme.ColorNameShadow:          color.RGBA{R: 0, G: 0, B: 0, A: 50},
	theme.ColorNameScrollBar:       color.RGBA{R: 180, G: 185, B: 200, A: 255},
	theme.ColorNameSeparator:       color.RGBA{R: 220, G: 222, B: 228, A: 255},
}

func (t *AppTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	var colors map[fyne.ThemeColorName]color.Color
	if t.themeType == ThemeDark {
		colors = darkColors
	} else {
		colors = lightColors
	}

	if c, ok := colors[name]; ok {
		return c
	}
	return theme.DefaultTheme().Color(name, variant)
}

func (t *AppTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (t *AppTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

func (t *AppTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding:
		return 6
	case theme.SizeNameInlineIcon:
		return 20
	case theme.SizeNameScrollBar:
		return 12
	case theme.SizeNameScrollBarSmall:
		return 4
	case theme.SizeNameText:
		return 14
	case theme.SizeNameHeadingText:
		return 20
	case theme.SizeNameSubHeadingText:
		return 16
	case theme.SizeNameCaptionText:
		return 12
	case theme.SizeNameInputBorder:
		return 2
	default:
		return theme.DefaultTheme().Size(name)
	}
}

// SetTheme change le thème de l'application
func SetTheme(t ThemeType) {
	currentTheme = t
	if myApp != nil {
		myApp.Settings().SetTheme(NewAppTheme(t))
	}
}

// GetCurrentTheme retourne le thème actuel
func GetCurrentTheme() ThemeType {
	return currentTheme
}

// ToggleTheme bascule entre les thèmes
func ToggleTheme() {
	if currentTheme == ThemeDark {
		SetTheme(ThemeLight)
	} else {
		SetTheme(ThemeDark)
	}
}

var currentTheme ThemeType = ThemeDark
