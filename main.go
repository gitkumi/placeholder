package main

import (
	"bytes"
	"log"
	"net/http"
	"os"

	"image"
	"image/color"
	"image/draw"
	"image/png"

	"github.com/gin-gonic/gin"
)

var environment = os.Getenv("ENVIRONMENT")

type Image struct {
	width int
	height int
	text string
	bgColor color.RGBA
	textColor color.RGBA
	data image.Image  
}

func (i *Image) setSize(size string) {
	i.width = 150
	i.height = 150
}

func (i *Image) setText(text string) {
	i.text = text
}

func (i *Image) generate() {
	bg := color.RGBA{200, 200, 200, 255} // Gray color
	data := image.NewRGBA(image.Rect(0, 0, i.width, i.height))
	draw.Draw(data, data.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)
	i.data = data
}

func main() {
	r := gin.Default()

	r.GET("/:size", func(c *gin.Context) {
		var img Image
		img.setSize(c.Param("size"))
		img.setText(c.Query("text"))
		img.generate()

		buffer := new(bytes.Buffer)
		err := png.Encode(buffer, img.data)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to encode image"})
			return
		}

		c.Data(http.StatusOK, "image/png", buffer.Bytes())
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
