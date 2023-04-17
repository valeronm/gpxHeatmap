package main

import (
	"fmt"
	"github.com/StephaneBunel/bresenham"
	"github.com/paulmach/orb"
	"github.com/paulmach/orb/maptile"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"runtime"
)

const outputDir = "D:/tiles"
const tileSize = 256
const minZoom = 0
const maxZoom = 16
const baseValue = 0
const alphaBaseValue = 64

type HeatTile [tileSize][tileSize]float64

func buildTiles(segments *[]*Segment) error {
	var zoom maptile.Zoom
	for zoom = minZoom; zoom <= maxZoom; zoom++ {
		fmt.Printf("Zoom: %d, Processing segments...\n", zoom)

		heatTiles := processSegments(segments, zoom)

		fmt.Printf("Zoom: %d, Searching maximum...\n", zoom)
		var max float64 = 0
		for _, tileHeat := range heatTiles {
			for x := 0; x < tileSize; x++ {
				for y := 0; y < tileSize; y++ {
					if max < tileHeat[x][y] {
						max = tileHeat[x][y]
					}
				}
			}
		}
		fmt.Printf("Zoom: %d, Maximum: %f\n", zoom, max)

		maxLog := math.Log(max * 10)
		counter := 0
		for key, tileHeat := range heatTiles {
			tile := maptile.FromQuadkey(key, zoom)
			fmt.Printf("Zoom: %d, Normalizing tile %d, %d\n", zoom, tile.X, tile.Y)

			tileImage := heatTileToGraphicLog(maxLog, tileHeat)

			var dirName = fmt.Sprintf("%s/%d/%d", outputDir, zoom, tile.X)
			err := os.MkdirAll(dirName, os.ModeDir+os.ModePerm)
			if err != nil {
				return err
			}

			var name = fmt.Sprintf("%s/%d/%d/%d.png", outputDir, zoom, tile.X, tile.Y)
			f, err := os.Create(name)
			if err != nil {
				return err
			}

			err = png.Encode(f, tileImage)
			if err != nil {
				return err
			}

			err = f.Close()
			if err != nil {
				return err
			}

			if counter == 200 {
				counter = 0
				runtime.GC()
			}
			counter += 1
		}
	}
	return nil
}

func processSegments(segments *[]*Segment, zoom maptile.Zoom) map[uint64]*HeatTile {
	heatTiles := make(map[uint64]*HeatTile)
	for _, segment := range *segments {
		fromLat := segment.from.lat
		fromLon := segment.from.lon
		var fromPoint = orb.Point{fromLon, fromLat}
		var fromTile = maptile.At(fromPoint, zoom)

		toLat := segment.to.lat
		toLon := segment.to.lon
		var toPoint = orb.Point{toLon, toLat}
		var toTile = maptile.At(toPoint, zoom)

		var heat = heatTiles[fromTile.Quadkey()]

		if heat == nil {
			heat = new(HeatTile)
			heatTiles[fromTile.Quadkey()] = heat
		}

		var fromPointInt = maptile.Fraction(fromPoint, zoom)
		var x1 = int(math.Floor((fromPointInt.X() - float64(fromTile.X)) * tileSize))
		var y1 = int(math.Floor((fromPointInt.Y() - float64(fromTile.Y)) * tileSize))

		var toPointInt = maptile.Fraction(toPoint, zoom)
		var x2 = int(math.Floor((toPointInt.X() - float64(fromTile.X)) * tileSize))
		var y2 = int(math.Floor((toPointInt.Y() - float64(fromTile.Y)) * tileSize))

		if x1 == x2 && y1 == y2 {
			heat.Set(x1, y1, color.Gray{})
		} else {
			bresenham.DrawLine(heat, x1, y1, x2, y2, color.Gray{Y: 1})
			heat.UnSet(x2, y2, color.Gray{})
		}

		if fromTile != toTile {

			heat = heatTiles[toTile.Quadkey()]

			if heat == nil {
				heat = new(HeatTile)
				heatTiles[toTile.Quadkey()] = heat
			}

			fromPointInt = maptile.Fraction(fromPoint, zoom)
			x1 = int(math.Floor((fromPointInt.X() - float64(toTile.X)) * tileSize))
			y1 = int(math.Floor((fromPointInt.Y() - float64(toTile.Y)) * tileSize))

			toPointInt = maptile.Fraction(toPoint, zoom)
			x2 = int(math.Floor((toPointInt.X() - float64(toTile.X)) * tileSize))
			y2 = int(math.Floor((toPointInt.Y() - float64(toTile.Y)) * tileSize))

			if x1 == x2 && y1 == y2 {
				heat.Set(x2, y2, color.Gray{})
			} else {
				bresenham.DrawLine(heat, x1, y1, x2, y2, color.Gray{Y: 1})
				heat.UnSet(x2, y2, color.Gray{})
			}
		}
	}
	return heatTiles
}

func heatTileToGraphicLog(maxLog float64, tile *HeatTile) *image.NRGBA {
	graphic := image.NewNRGBA(image.Rect(0, 0, tileSize, tileSize))
	for x := 0; x < tileSize; x++ {
		for y := 0; y < tileSize; y++ {
			heat := tile[x][y]
			if heat > 0 {
				pix := math.Log(heat*10) / maxLog
				normalized := baseValue + pix*(255-baseValue)
				alphaNormalized := alphaBaseValue + pix*(255-alphaBaseValue)
				intNormalized := uint8(normalized)

				pixColor := color.NRGBA{R: 255 - intNormalized, G: intNormalized, B: 0, A: uint8(alphaNormalized)}
				graphic.SetNRGBA(x, y, pixColor)
			}
		}
	}
	return graphic
}

func (h *HeatTile) Set(x int, y int, c color.Color) {
	if x >= 0 && y >= 0 && x < tileSize && y < tileSize {
		h[x][y] += 1
	}
}

func (h *HeatTile) UnSet(x int, y int, c color.Color) {
	if x >= 0 && y >= 0 && x < tileSize && y < tileSize {
		h[x][y] -= 1
	}
}
