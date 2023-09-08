package main

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/math/fixed"
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

func main() {
	r := gin.Default()
	r.GET("/:size", imageHandler)
	port := ternary(environment == "production", ":8080", ":3000")
	if err := r.Run(port); err != nil {
		log.Fatal(err)
	}
}

func imageHandler(c *gin.Context) {
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
}

func (i *Image) setSize(size string) {
	dimensions := strings.Split(size, "x")
	i.width, i.height = parseDimensions(dimensions)
}

func parseDimensions(dimensions []string) (int, int) {
	width, height := 150, 150
	switch len(dimensions) {
	case 2:
		w, err := strconv.Atoi(dimensions[0])
		if err == nil {
			width = w
		}
		h, err := strconv.Atoi(dimensions[1])
		if err == nil {
			height = h
		}
	case 1:
		s, err := strconv.Atoi(dimensions[0])
		if err == nil {
			width = s
			height = s
		}
	}
	return clamp(width, 150, 3000), clamp(height, 150, 3000)
}

func (i *Image) setColors(hexBg, hexFg string) {
	i.bg = parseHexColor(hexBg, color.RGBA{0xD4, 0xD4, 0xD4, 0xFF})
	i.fg = parseHexColor(hexFg, color.RGBA{0x73, 0x73, 0x73, 0xFF})
}

func parseHexColor(hex string, defaultColor color.RGBA) color.RGBA {
	if len(hex) == 0 {
		return defaultColor
	}

	if hex[0] == '#' {
		hex = hex[1:]
	}

	if len(hex) >= 3 && len(hex) <= 8 {
		if len(hex) == 3 {
			hex = strings.Repeat(hex, 2)
		}

		if rgba, err := hexToRGBA(hex); err == nil {
			return rgba
		}
	}

	return defaultColor
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
	i.fontSize = parseFontSize(font, float64(i.width)/5)
}

func parseFontSize(font string, defaultSize float64) float64 {
	if size, err := strconv.ParseFloat(font, 64); err == nil {
		return size
	}
	return defaultSize
}

func (i *Image) apply() error {
	img := image.NewRGBA(image.Rect(0, 0, i.width, i.height))
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

	padding := 30
	lines := wrapText(i.text, fontDrawer, float64(i.width-padding))

	totalTextHeight := fixed.I(0)
	for _, line := range lines {
		textBounds, _ := fontDrawer.BoundString(line)
		textHeight := textBounds.Max.Y - textBounds.Min.Y
		totalTextHeight += textHeight
	}

	// Calculate the starting yPosition to center the text vertically
	yPosition := (fixed.I(img.Rect.Max.Y) - totalTextHeight) / 2

	// Draw each line of text
	for _, line := range lines {
		textBounds, _ := fontDrawer.BoundString(line)
		xPosition := (fixed.I(img.Rect.Max.X) - fontDrawer.MeasureString(line)) / 2
		textHeight := textBounds.Max.Y - textBounds.Min.Y

		// Adjust yPosition for each line
		yPosition += textHeight

		fontDrawer.Dot = fixed.Point26_6{
			X: xPosition,
			Y: yPosition,
		}

		fontDrawer.DrawString(line)
	}

	i.data = img

	return nil
}

func wrapText(text string, drawer *font.Drawer, maxWidth float64) []string {
	var lines []string
	var currentLine string
	var currentWidth float64

	words := strings.Fields(text)

	for _, word := range words {
		testLine := currentLine
		if len(testLine) > 0 {
			testLine += " "
		}
		testLine += word
		currentWidth = float64(drawer.MeasureString(testLine) / 64.0)

		if currentWidth > maxWidth {
			if len(currentLine) > 0 {
				lines = append(lines, currentLine)
			}
			currentLine = word
		} else {
			if len(currentLine) > 0 {
				currentLine += " "
			}
			currentLine += word
		}
	}

	if len(currentLine) > 0 {
		lines = append(lines, currentLine)
	}

	return lines
}

func (i *Image) generate() ([]byte, error) {
	buffer := new(bytes.Buffer)
	err := png.Encode(buffer, i.data)
	return buffer.Bytes(), err
}

func clamp(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func ternary[T any](cond bool, left, right T) T {
	if cond {
		return left
	}
	return right
}
