package main

import (
	"fmt"
	"github.com/StephaneBunel/bresenham"
	"github.com/paulmach/orb"
	"github.com/paulmach/orb/maptile"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"math/rand"
	"os"
	"runtime"
)

const tileSize = 256
const minZoom = 0
const maxZoom = 16
const baseValue = 0

type HeatTile [tileSize][tileSize]float64
type HeatTileSet map[uint64]*HeatTile
type ZoomHeatTileSet map[int]*HeatTileSet

func buildTiles(outputDir string, segments *Segments) error {
	err := buildEmptyTile(outputDir)
	if err != nil {
		return err
	}

	for zoom := minZoom; zoom <= maxZoom; zoom++ {
		fmt.Printf("Zoom: %d, Processing segments...\n", zoom)

		heatTiles := processSegments(segments, maptile.Zoom(zoom))

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
			tile := maptile.FromQuadkey(key, maptile.Zoom(zoom))
			fmt.Printf("Zoom: %d, Normalizing tile %d, %d\n", zoom, tile.X, tile.Y)

			tileImage := heatTileToGraphicLog(int(zoom), maxLog, tileHeat)

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

func toPoint(p orb.Point) *Point {
	return &Point{lat: p.Lat(), lon: p.Lon()}
}

func (a App) buildTile(outputDir string, zoom int, x int, y int) error {
	mapTile := maptile.New(uint32(x), uint32(y), maptile.Zoom(zoom))
	bound := mapTile.Bound()

	segments, err := a.repo.segmentsForRange(toPoint(bound.Min), toPoint(bound.Max))
	if err != nil {
		return err
	}

	if len(*segments) == 0 {
		return nil
	}

	fmt.Printf("Drawing %d segments...\n", len(*segments))
	heatTiles := processSegments(segments, maptile.Zoom(zoom))
	heatTile := heatTiles[mapTile.Quadkey()]

	if heatTile == nil {
		return nil
	}

	fmt.Printf("Searching tile maximum...\n")
	var max float64 = 0
	for x := 0; x < tileSize; x++ {
		for y := 0; y < tileSize; y++ {
			if max < heatTile[x][y] {
				max = heatTile[x][y]
			}
		}
	}
	fmt.Printf("Maximum: %f\n", max)

	fmt.Printf("Normalizing tile\n")
	maxLog := math.Log(max * 10)
	tileImage := heatTileToGraphicLog(zoom, maxLog, heatTile)

	var dirName = fmt.Sprintf("%s/%d/%d", outputDir, zoom, x)
	err = os.MkdirAll(dirName, os.ModeDir+os.ModePerm)
	if err != nil {
		return err
	}

	var name = fmt.Sprintf("%s/%d/%d/%d.png", outputDir, zoom, x, y)
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
	return nil
}

func buildEmptyTile(outputDir string) error {
	tileImage := emptyTile()

	var name = fmt.Sprintf("%s/empty.png", outputDir)
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
	return nil
}

const random = 0.00002

func processSegments(segments *Segments, zoom maptile.Zoom) HeatTileSet {
	heatTiles := make(HeatTileSet)
	for _, segment := range *segments {
		fromLat := segment.from.lat + (rand.Float64() * random * 2) - random
		fromLon := segment.from.lon + (rand.Float64() * random * 2) - random
		var fromPoint = orb.Point{fromLon, fromLat}
		var fromTile = maptile.At(fromPoint, zoom)

		toLat := segment.to.lat + (rand.Float64() * random * 2) - random
		toLon := segment.to.lon + (rand.Float64() * random * 2) - random
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

func heatTileToGraphicLog(zoom int, maxLog float64, tile *HeatTile) *image.Gray {
	graphic := emptyTile()
	for x := 0; x < tileSize; x++ {
		for y := 0; y < tileSize; y++ {
			heat := tile[x][y]
			if heat > 0 {
				pix := math.Log(heat*float64(zoom+1)/2) / maxLog
				normalized := baseValue + pix*(255-baseValue)
				intNormalized := uint8(normalized)

				pixColor := color.Gray{Y: intNormalized}
				graphic.SetGray(x, y, pixColor)
			}
		}
	}
	return graphic
}

func emptyTile() *image.Gray {
	graphic := image.NewGray(image.Rect(0, 0, tileSize, tileSize))
	bg := color.Gray{}
	draw.Draw(graphic, graphic.Bounds(), &image.Uniform{C: bg}, image.ZP, draw.Src)
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
