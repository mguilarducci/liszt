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

func TestThemeStylesProduceOutput(t *testing.T) {
	t.Parallel()

	if styH1.Render("X") == "" {
		t.Errorf("styH1 produced empty output")
	}
	if styH2.Render("Y") == "" {
		t.Errorf("styH2 produced empty output")
	}
	if styH3.Render("Z") == "" {
		t.Errorf("styH3 produced empty output")
	}
	if styPink.Render("W") == "" {
		t.Errorf("styPink produced empty output")
	}
}

func TestLabelConstantsSameWidth(t *testing.T) {
	t.Parallel()

	labels := []string{lblDone, lblWarn, lblStep}
	want := len(lblDone)
	for _, l := range labels {
		if len(l) != want {
			t.Errorf("label %q has width %d; want %d", l, len(l), want)
		}
	}
}
