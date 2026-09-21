package main

import (
	"image/png"
	"os"
	"testing"
)

func TestIcon(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "icon-*.png")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	old := os.Stdout
	os.Stdout = f
	defer func() { os.Stdout = old }()
	main()
	if _, err := f.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 256 || img.Bounds().Dy() != 256 {
		t.Fatal(img.Bounds())
	}
	if img.At(0, 0) == img.At(72, 32) {
		t.Fatal("missing rail")
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if recover() == nil {
			t.Error("expected write failure")
		}
	}()
	main()
}
