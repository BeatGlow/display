package pixel

import (
	"image"
	"image/color"
	"math/rand"
	"testing"
)

func TestMonoImage(t *testing.T) {
	testImage(t, func(size image.Point) Image {
		return NewMonoImage(size.X, size.Y)
	}, MonoModel)
}

func TestMonoVerticalLSBImageImage(t *testing.T) {
	testImage(t, func(size image.Point) Image {
		return NewMonoVerticalLSBImage(size.X, size.Y)
	}, MonoModel)
}

func TestGray2Image(t *testing.T) {
	testImage(t, func(size image.Point) Image {
		return NewGray2Image(size.X, size.Y)
	}, Gray2Model)
}

func TestGray4Image(t *testing.T) {
	testImage(t, func(size image.Point) Image {
		return NewGray4Image(size.X, size.Y)
	}, Gray4Model)
}

func TestCBGR15Image(t *testing.T) {
	testImage(t, func(size image.Point) Image {
		return NewCBGR15Image(size.X, size.Y)
	}, CBGR15Model)
}

func TestCBGR16Image(t *testing.T) {
	testImage(t, func(size image.Point) Image {
		return NewCBGR16Image(size.X, size.Y)
	}, CBGR16Model)
}

func TestCRGB15Image(t *testing.T) {
	testImage(t, func(size image.Point) Image {
		return NewCRGB15Image(size.X, size.Y)
	}, CRGB15Model)
}

func TestCRGB16Image(t *testing.T) {
	testImage(t, func(size image.Point) Image {
		return NewCRGB16Image(size.X, size.Y)
	}, CRGB16Model)
}

func testImage(t *testing.T, f func(image.Point) Image, model color.Model) {
	t.Helper()
	testCases := []image.Point{
		image.Point{},
		image.Pt(1, 1),
		image.Pt(2, 2),
		image.Pt(256, 32),
		image.Pt(256, 64),
	}
	for _, test := range testCases {
		t.Run(test.String(), func(it *testing.T) {
			i := f(test)

			if v := i.Bounds().Size(); !v.Eq(test) {
				it.Errorf("expected image size %s, got %s", test, v)
			}

			if v := i.ColorModel(); v != model {
				it.Errorf("expected color model %T, got %T", model, v)
			}

			it.Run("in-bounds", func(itt *testing.T) {
				for y := 0; y < test.Y; y++ {
					for x := 0; x < test.X; x++ {
						c := testRandomColor()
						i.Set(x, y, c)
						if v := i.ColorModel().Convert(c); i.At(x, y) != v {
							itt.Fatalf("pixel (%d,%d) is %#+v, expected %#+v (%v)", x, y, i.At(x, y), v, c)
							return
						}
					}
				}
			})

			it.Run("in-bounds-matching-model", func(itt *testing.T) {
				for y := 0; y < test.Y; y++ {
					for x := 0; x < test.X; x++ {
						c := model.Convert(testRandomColor())
						i.Set(x, y, c)
						if i.At(x, y) != c {
							itt.Fatalf("pixel (%d,%d) is %#+v, expected %#+v", x, y, i.At(x, y), c)
							return
						}
					}
				}
			})

			it.Run("out-bounds", func(itt *testing.T) {
				for y := -test.Y; y < test.Y*2; y++ {
					for x := -test.X; x < test.X*2; x++ {
						i.Set(x, y, testRandomColor())
						if x < 0 || y < 0 {
							if v := i.At(x, y); v != color.Transparent {
								itt.Fatalf("pixel (%d,%d) is %#+v, expected transparent", x, y, v)
								return
							}
						}
					}
				}
			})

			it.Run("fill", func(itt *testing.T) {
				c := testRandomColor()
				i.Fill(c)
				if test.X > 0 && test.Y > 0 {
					x := rand.Intn(test.X)
					y := rand.Intn(test.Y)
					if v := i.ColorModel().Convert(c); i.At(x, y) != v {
						itt.Fatalf("pixel (%d,%d) is %#+v, expected %#+v (%v)", x, y, i.At(x, y), v, c)
						return
					}
				}
			})

			it.Run("clear", func(itt *testing.T) {
				i.Clear()
				if test.X > 0 && test.Y > 0 {
					x := rand.Intn(test.X)
					y := rand.Intn(test.Y)
					if v := monoModel(i.At(x, y)); v != Off {
						itt.Fatalf("pixel (%d,%d) is not black", x, y)
					}
				}
			})
		})
	}
}

func testRandomColor() color.Color {
	return color.RGBA{
		R: uint8(rand.Intn(255)),
		G: uint8(rand.Intn(255)),
		B: uint8(rand.Intn(255)),
		A: 0xFF,
	}
}
