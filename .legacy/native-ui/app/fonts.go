package app

import (
	_ "embed"
	"log"

	"github.com/egoist/mygo/ui"
)

// notoSC is the CJK font the console's Chinese labels need. Few Linux
// installations ship a CJK family, and MyGo draws text with the fonts it
// knows, so the app carries one and registers it under uiFontFamily.
//
//go:embed assets/NotoSansSC-Regular.otf
var notoSC []byte

// uiFontFamily is the family text draws in: Noto Sans SC, which covers both
// Latin and CJK, so a single font serves the whole interface.
const uiFontFamily = "Noto Sans SC"

// registerFonts registers the embedded CJK font once. It is safe to call
// from every window and from tests; a second registration is harmless but
// skipped after the first succeeds.
var fontsRegistered bool

func registerFonts() {
	if fontsRegistered {
		return
	}
	if err := ui.RegisterFont(notoSC, uiFontFamily); err != nil {
		log.Printf("register CJK font: %v", err)
		return
	}
	fontsRegistered = true
}

// ensureFonts registers the fonts for a view about to build. Kept separate
// from registerFonts so the view can call it every frame cheaply.
func ensureFonts() { registerFonts() }
