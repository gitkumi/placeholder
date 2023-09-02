package main

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"image"
	"image/color"
	"image/draw"
	"image/png"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/math/fixed"

	"github.com/gin-gonic/gin"
	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
)

var environment = os.Getenv("ENVIRONMENT")

type Image struct {
	width    int
	height   int
	text     string
	fontSize float64
	bg       color.RGBA
	fg       color.RGBA
	data     *image.RGBA
}

func (i *Image) setSize(size string) {
	var width int
	var height int

	dimensions := strings.Split(size, "x")

	switch len(dimensions) {
	case 2:
		w, err := strconv.Atoi(dimensions[0])
		width = ternary(err == nil, w, 150)
		h, err := strconv.Atoi(dimensions[1])
		height = ternary(err == nil, h, 150)
	case 1:
		s, err := strconv.Atoi(dimensions[0])
		width = ternary(err == nil, s, 150)
		height = ternary(err == nil, s, 150)
	default:
		width = 150
		height = 150
	}

	if width > 3000 {
		width = 3000
	}

	if height > 3000 {
		height = 3000
	}

	i.width = width
	i.height = height
}

func (i *Image) setColors(hexBg, hexFg string) {
	// d4d4d4
	i.bg = color.RGBA{0xD4, 0xD4, 0xD4, 0xFF}
	// 737373
	i.fg = color.RGBA{0x73, 0x73, 0x73, 0xFF}

	if len(hexBg) > 0 {
		if rgbaBg, err := hexToRGBA(hexBg); err == nil {
			i.bg = rgbaBg
		}
	}

	if len(hexFg) > 0 {
		if rgbaFg, err := hexToRGBA(hexFg); err == nil {
			i.fg = rgbaFg
		}
	}
}

func hexToRGBA(hex string) (color.RGBA, error) {
	// Remove the '#' symbol if it's included
	if hex[0] == '#' {
		hex = hex[1:]
	}

	if len(hex) <= 2 || len(hex) >= 9 {
		return color.RGBA{}, errors.New("Hex should be 3-8 characters.")
	}

	if len(hex) == 3 {
		var duplicated strings.Builder

		for _, char := range hex {
			duplicated.WriteString(string(char))
			duplicated.WriteString(string(char))
		}

		hex = duplicated.String()
	}

	r, err := strconv.ParseUint(hex[0:2], 16, 0)
	if err != nil {
		return color.RGBA{}, err
	}

	g, err := strconv.ParseUint(hex[2:4], 16, 0)
	if err != nil {
		return color.RGBA{}, err
	}

	b, err := strconv.ParseUint(hex[4:6], 16, 0)
	if err != nil {
		return color.RGBA{}, err
	}

	// 255 alpha as default
	a := uint64(255)
	if len(hex) == 8 {
		alpha, err := strconv.ParseUint(hex[6:10], 16, 0)

		if err != nil {
			return color.RGBA{}, err
		}

		a = alpha
	}

	return color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: uint8(a)}, nil
}

func (i *Image) setText(text string) {
	if len(text) > 0 {
		i.text = text
	} else {
		i.text = fmt.Sprintf("%dx%d", i.width, i.height)
	}
}

func (i *Image) setFont(font string) {
	i.fontSize = float64(i.height) / 5

	if size, err := strconv.ParseFloat(font, 64); err == nil {
		i.fontSize = size
	}
}

func (i *Image) apply() error {
	upLeft := image.Point{0, 0}
	lowRight := image.Point{i.width, i.height}
	img := image.NewRGBA(image.Rectangle{upLeft, lowRight})
	draw.Draw(img, img.Bounds(), &image.Uniform{i.bg}, image.Point{}, draw.Src)

	// Add text
	fontFace, err := freetype.ParseFont(goregular.TTF)
	if err != nil {
		return errors.New("Cannot parse font.")
	}
	fontDrawer := &font.Drawer{
		Dst: img,
		Src: &image.Uniform{i.fg},
		Face: truetype.NewFace(fontFace, &truetype.Options{
			Size:    i.fontSize,
			Hinting: font.HintingFull,
		}),
	}
	textBounds, _ := fontDrawer.BoundString(i.text)
	xPosition := (fixed.I(img.Rect.Max.X) - fontDrawer.MeasureString(i.text)) / 2
	textHeight := textBounds.Max.Y - textBounds.Min.Y
	yPosition := fixed.I((img.Rect.Max.Y)-textHeight.Ceil())/2 + fixed.I(textHeight.Ceil())
	fontDrawer.Dot = fixed.Point26_6{
		X: xPosition,
		Y: yPosition,
	}
	fontDrawer.DrawString(i.text)
	i.data = img

	return nil
}

func (i *Image) generate() ([]byte, error) {
	buffer := new(bytes.Buffer)
	err := png.Encode(buffer, i.data)

	return buffer.Bytes(), err
}

func main() {
	r := gin.Default()

	r.GET("/:size", func(c *gin.Context) {
		img := &Image{}
		img.setSize(c.Param("size"))
		img.setFont(c.Query("fontSize"))
		img.setText(c.Query("text"))
		img.setColors(c.Query("bg"), c.Query("fg"))
		err := img.apply()

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create an image."})
			return
		}

		bytes, err := img.generate()

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to encode the image."})
			return
		}

		c.Data(http.StatusOK, "image/png", bytes)
	})

	port := ternary(environment == "production", ":8080", ":3000")

	if err := r.Run(port); err != nil {
		log.Fatal(err)
	}
}

func ternary[T any](cond bool, left, right T) T {
	if cond {
		return left
	}

	return right
}
