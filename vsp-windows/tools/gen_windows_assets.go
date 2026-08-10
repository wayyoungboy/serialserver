//go:build ignore

package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

var (
	bgTop       = color.RGBA{R: 22, G: 36, B: 48, A: 255}
	bgBottom    = color.RGBA{R: 13, G: 20, B: 28, A: 255}
	cyan        = color.RGBA{R: 42, G: 202, B: 218, A: 255}
	green       = color.RGBA{R: 82, G: 204, B: 132, A: 255}
	orange      = color.RGBA{R: 242, G: 143, B: 58, A: 255}
	white       = color.RGBA{R: 242, G: 247, B: 250, A: 255}
	transparent = color.RGBA{}
)

func main() {
	must(os.MkdirAll("build/windows", 0755))

	appIcon := renderIcon(1024)
	must(writePNG("build/appicon.png", appIcon))
	must(writeICO("build/windows/icon.ico", []int{16, 24, 32, 48, 64, 128, 256}))
	must(os.Chmod("build/appicon.png", 0644))
	must(os.Chmod("build/windows/icon.ico", 0644))
}

func renderIcon(size int) image.Image {
	scale := 4
	canvas := image.NewRGBA(image.Rect(0, 0, size*scale, size*scale))
	drawIcon(canvas)
	return downsample(canvas, size, size)
}

func drawIcon(img *image.RGBA) {
	w := img.Bounds().Dx()
	h := img.Bounds().Dy()
	r := float64(w) * 0.22

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if inRoundRect(float64(x)+0.5, float64(y)+0.5, 0, 0, float64(w), float64(h), r) {
				t := float64(y) / float64(h-1)
				img.SetRGBA(x, y, lerp(bgTop, bgBottom, t))
			}
		}
	}

	cx := float64(w) * 0.5
	cy := float64(h) * 0.5
	rail := float64(w) * 0.07

	thickLine(img, float64(w)*0.23, float64(h)*0.32, cx, float64(h)*0.68, rail, cyan)
	thickLine(img, cx, float64(h)*0.68, float64(w)*0.77, float64(h)*0.32, rail, green)
	thickLine(img, float64(w)*0.28, cy, float64(w)*0.72, cy, float64(w)*0.055, color.RGBA{R: 235, G: 241, B: 245, A: 235})

	for _, x := range []float64{0.25, 0.36, 0.64, 0.75} {
		circle(img, float64(w)*x, cy, float64(w)*0.045, orange)
		circle(img, float64(w)*x, cy, float64(w)*0.024, white)
	}

	circle(img, cx, cy, float64(w)*0.078, orange)
	circle(img, cx, cy, float64(w)*0.04, white)
}

func inRoundRect(x, y, left, top, right, bottom, radius float64) bool {
	if x < left || x >= right || y < top || y >= bottom {
		return false
	}
	cx := math.Min(math.Max(x, left+radius), right-radius)
	cy := math.Min(math.Max(y, top+radius), bottom-radius)
	return math.Hypot(x-cx, y-cy) <= radius
}

func thickLine(img *image.RGBA, x1, y1, x2, y2, width float64, c color.RGBA) {
	minX := int(math.Max(0, math.Floor(math.Min(x1, x2)-width)))
	maxX := int(math.Min(float64(img.Bounds().Dx()-1), math.Ceil(math.Max(x1, x2)+width)))
	minY := int(math.Max(0, math.Floor(math.Min(y1, y2)-width)))
	maxY := int(math.Min(float64(img.Bounds().Dy()-1), math.Ceil(math.Max(y1, y2)+width)))
	dx := x2 - x1
	dy := y2 - y1
	lengthSquared := dx*dx + dy*dy

	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			px := float64(x) + 0.5
			py := float64(y) + 0.5
			t := ((px-x1)*dx + (py-y1)*dy) / lengthSquared
			t = math.Max(0, math.Min(1, t))
			nearestX := x1 + t*dx
			nearestY := y1 + t*dy
			if math.Hypot(px-nearestX, py-nearestY) <= width/2 {
				alphaComposite(img, x, y, c)
			}
		}
	}
}

func circle(img *image.RGBA, cx, cy, radius float64, c color.RGBA) {
	minX := int(math.Max(0, math.Floor(cx-radius)))
	maxX := int(math.Min(float64(img.Bounds().Dx()-1), math.Ceil(cx+radius)))
	minY := int(math.Max(0, math.Floor(cy-radius)))
	maxY := int(math.Min(float64(img.Bounds().Dy()-1), math.Ceil(cy+radius)))
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			if math.Hypot(float64(x)+0.5-cx, float64(y)+0.5-cy) <= radius {
				alphaComposite(img, x, y, c)
			}
		}
	}
}

func alphaComposite(img *image.RGBA, x, y int, fg color.RGBA) {
	bg := img.RGBAAt(x, y)
	a := float64(fg.A) / 255
	img.SetRGBA(x, y, color.RGBA{
		R: uint8(float64(fg.R)*a + float64(bg.R)*(1-a)),
		G: uint8(float64(fg.G)*a + float64(bg.G)*(1-a)),
		B: uint8(float64(fg.B)*a + float64(bg.B)*(1-a)),
		A: uint8(math.Min(255, float64(fg.A)+float64(bg.A)*(1-a))),
	})
}

func lerp(a, b color.RGBA, t float64) color.RGBA {
	return color.RGBA{
		R: uint8(float64(a.R)*(1-t) + float64(b.R)*t),
		G: uint8(float64(a.G)*(1-t) + float64(b.G)*t),
		B: uint8(float64(a.B)*(1-t) + float64(b.B)*t),
		A: 255,
	}
}

func downsample(src *image.RGBA, width, height int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	sx := src.Bounds().Dx() / width
	sy := src.Bounds().Dy() / height
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			var r, g, b, a int
			for yy := 0; yy < sy; yy++ {
				for xx := 0; xx < sx; xx++ {
					c := src.RGBAAt(x*sx+xx, y*sy+yy)
					r += int(c.R)
					g += int(c.G)
					b += int(c.B)
					a += int(c.A)
				}
			}
			count := sx * sy
			dst.SetRGBA(x, y, color.RGBA{
				R: uint8(r / count),
				G: uint8(g / count),
				B: uint8(b / count),
				A: uint8(a / count),
			})
		}
	}
	return dst
}

func writePNG(path string, img image.Image) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return png.Encode(file, img)
}

func writeICO(path string, sizes []int) error {
	var entries [][]byte
	for _, size := range sizes {
		var buf bytes.Buffer
		if err := png.Encode(&buf, renderIcon(size)); err != nil {
			return err
		}
		entries = append(entries, buf.Bytes())
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	must(binary.Write(file, binary.LittleEndian, uint16(0)))
	must(binary.Write(file, binary.LittleEndian, uint16(1)))
	must(binary.Write(file, binary.LittleEndian, uint16(len(entries))))

	offset := 6 + 16*len(entries)
	for i, data := range entries {
		size := sizes[i]
		sizeByte := byte(size)
		if size >= 256 {
			sizeByte = 0
		}
		file.Write([]byte{sizeByte, sizeByte, 0, 0})
		must(binary.Write(file, binary.LittleEndian, uint16(1)))
		must(binary.Write(file, binary.LittleEndian, uint16(32)))
		must(binary.Write(file, binary.LittleEndian, uint32(len(data))))
		must(binary.Write(file, binary.LittleEndian, uint32(offset)))
		offset += len(data)
	}
	for _, data := range entries {
		_, err := file.Write(data)
		if err != nil {
			return err
		}
	}
	return nil
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
