// Command appicon draws lmmtfy Studio's icon: ragged source columns on the
// left, an arrow, tidy aligned columns on the right — messy CSV in, fixed
// format out. Run it from the repository root to regenerate the PNG Wails
// turns into the macOS and Windows icons:
//
//	go run ./tools/appicon -o studio/build/appicon.png
package main

import (
	"flag"
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"
)

const (
	size = 1024
	ss   = 4 // supersampling factor, for smooth edges
)

func main() {
	out := flag.String("o", "appicon.png", "output PNG file")
	flag.Parse()

	big := image.NewRGBA(image.Rect(0, 0, size*ss, size*ss))
	s := float64(ss)

	// Background: rounded square with a vertical blue gradient.
	top, bottom := color.RGBA{0x3d, 0x7f, 0xf0, 0xff}, color.RGBA{0x1f, 0x4f, 0xc4, 0xff}
	fill(big, func(x, y float64) (color.RGBA, bool) {
		if !inRoundedRect(x, y, 64*s, 64*s, 960*s, 960*s, 200*s) {
			return color.RGBA{}, false
		}
		return mix(top, bottom, (y/s-64)/896), true
	})

	white := color.RGBA{0xff, 0xff, 0xff, 0xff}
	faint := color.RGBA{0xff, 0xff, 0xff, 0x8c}
	// Ragged bars on the left: different lengths and offsets.
	for i, bar := range [][2]float64{{190, 360}, {230, 420}, {170, 330}, {210, 400}} {
		y := 300 + float64(i)*120
		roundRect(big, bar[0]*s, y*s, bar[1]*s, (y+56)*s, 28*s, faint)
	}
	// Aligned bars on the right.
	for i := range 4 {
		y := 300 + float64(i)*120
		roundRect(big, 620*s, y*s, 840*s, (y+56)*s, 28*s, white)
	}
	// Arrow in the middle.
	roundRect(big, 450*s, 492*s, 560*s, 532*s, 20*s, white)
	fill(big, func(x, y float64) (color.RGBA, bool) {
		x, y = x/s, y/s
		// Triangle pointing right: tip at (600, 512).
		return white, x >= 530 && x <= 600 && math.Abs(y-512) <= (600-x)*0.85
	})

	small := downscale(big)
	f, err := os.Create(*out)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, small); err != nil {
		log.Fatal(err)
	}
}

func fill(img *image.RGBA, shade func(x, y float64) (color.RGBA, bool)) {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if c, ok := shade(float64(x)+0.5, float64(y)+0.5); ok {
				img.SetRGBA(x, y, over(c, img.RGBAAt(x, y)))
			}
		}
	}
}

func roundRect(img *image.RGBA, x0, y0, x1, y1, r float64, c color.RGBA) {
	fill(img, func(x, y float64) (color.RGBA, bool) {
		return c, inRoundedRect(x, y, x0, y0, x1, y1, r)
	})
}

func inRoundedRect(x, y, x0, y0, x1, y1, r float64) bool {
	if x < x0 || x > x1 || y < y0 || y > y1 {
		return false
	}
	cx := math.Max(x0+r, math.Min(x, x1-r))
	cy := math.Max(y0+r, math.Min(y, y1-r))
	return math.Hypot(x-cx, y-cy) <= r
}

// over composites c (straight alpha) over dst.
func over(c, dst color.RGBA) color.RGBA {
	a := float64(c.A) / 255
	blend := func(s, d uint8) uint8 { return uint8(float64(s)*a + float64(d)*(1-a) + 0.5) }
	return color.RGBA{blend(c.R, dst.R), blend(c.G, dst.G), blend(c.B, dst.B), uint8(math.Min(255, float64(c.A)+float64(dst.A)*(1-a)))}
}

func mix(a, b color.RGBA, t float64) color.RGBA {
	t = math.Max(0, math.Min(1, t))
	m := func(x, y uint8) uint8 { return uint8(float64(x)*(1-t) + float64(y)*t + 0.5) }
	return color.RGBA{m(a.R, b.R), m(a.G, b.G), m(a.B, b.B), 0xff}
}

// downscale averages ss×ss blocks (premultiplied, so edges stay clean).
func downscale(big *image.RGBA) *image.NRGBA {
	out := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := range size {
		for x := range size {
			var r, g, b, a float64
			for dy := range ss {
				for dx := range ss {
					c := big.RGBAAt(x*ss+dx, y*ss+dy)
					w := float64(c.A) / 255
					r, g, b, a = r+float64(c.R)*w, g+float64(c.G)*w, b+float64(c.B)*w, a+w
				}
			}
			if a == 0 {
				continue
			}
			out.SetNRGBA(x, y, color.NRGBA{uint8(r/a + 0.5), uint8(g/a + 0.5), uint8(b/a + 0.5), uint8(a/float64(ss*ss)*255 + 0.5)})
		}
	}
	return out
}
