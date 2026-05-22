package render

import (
	"image/color"
	"testing"
)

func TestThemeColorsNotNil(t *testing.T) {
	t.Parallel()

	cases := map[string]color.Color{
		"cPinkDeep":   cPinkDeep,
		"cPinkBright": cPinkBright,
		"cInfo":       cInfo,
		"cDone":       cDone,
		"cWarn":       cWarn,
		"cError":      cError,
		"cDim":        cDim,
	}
	for name, c := range cases {
		if c == nil {
			t.Errorf("%s: expected non-nil color", name)
		}
	}
}

func TestPaletteMapsColors(t *testing.T) {
	t.Parallel()

	cases := map[string][2]color.Color{
		"PinkDeep":   {Palette.PinkDeep, cPinkDeep},
		"PinkBright": {Palette.PinkBright, cPinkBright},
		"Info":       {Palette.Info, cInfo},
		"Done":       {Palette.Done, cDone},
		"Warn":       {Palette.Warn, cWarn},
		"Error":      {Palette.Error, cError},
		"Dim":        {Palette.Dim, cDim},
	}
	for name, pair := range cases {
		if pair[0] != pair[1] {
			t.Errorf("Palette.%s = %v; want %v", name, pair[0], pair[1])
		}
	}
}
