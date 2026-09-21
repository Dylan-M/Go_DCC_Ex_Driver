// androidicon generates the test APK's simple railway-track icon.
package main

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
)

func main() {
	icon := image.NewRGBA(image.Rect(0, 0, 256, 256))
	draw.Draw(icon, icon.Bounds(), image.NewUniform(color.RGBA{33, 41, 61, 255}), image.Point{}, draw.Src)
	for y := 48; y <= 208; y += 32 {
		draw.Draw(icon, image.Rect(48, y, 208, y+12), image.NewUniform(color.RGBA{133, 153, 181, 255}), image.Point{}, draw.Src)
	}
	for _, x := range []int{72, 172} {
		draw.Draw(icon, image.Rect(x, 32, x+12, 224), image.NewUniform(color.RGBA{219, 235, 250, 255}), image.Point{}, draw.Src)
	}
	if err := png.Encode(os.Stdout, icon); err != nil {
		panic(err)
	}
}
