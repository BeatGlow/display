package scope_test

import (
	_ "embed"
	"fmt"
	"image"
	"strings"
	"time"

	"github.com/BeatGlow/display/draw/scope"
)

//go:embed testdata/beatglow.svg
var beatglowSplash string

func ExampleRenderer() {
	dst := image.NewRGBA(image.Rect(0, 0, 400, 128))
	cfg := scope.DefaultConfig()
	cfg.CRTBulge = 0.31
	renderer, err := scope.NewRenderer(dst.Bounds(), cfg)
	if err != nil {
		panic(err)
	}
	if err = renderer.CompileSVG(strings.NewReader(beatglowSplash)); err != nil {
		panic(err)
	}

	// In an application, use measured elapsed time in the display loop.
	renderer.UpdatePhysics(time.Second / 60)
	if err = renderer.RenderTo(dst); err != nil {
		panic(err)
	}
	// With a display.Display, BlitToDisplay also calls its Refresh method.
	fmt.Println(dst.Bounds())
	// Output: (0,0)-(400,128)
}
