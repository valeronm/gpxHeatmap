package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"math/rand"
	"os"
	"path/filepath"

	"github.com/StephaneBunel/bresenham"
	"github.com/paulmach/orb"
	"github.com/paulmach/orb/maptile"
	"github.com/tkrajina/gpxgo/gpx"
)

const maxZoom = 18

type HeatTile [256][256]float64
type Point struct {
	lat float64
	lon float64
}
type Segment struct {
	from Point
	to   Point
}

var segments []*Segment

func main() {
	flag.Parse()

	args := flag.Args()
	if len(args) != 1 {
		fmt.Println("Provide a GPX file path!")
		return
	}

	gpxFolder := args[0]
	err := filepath.Walk(gpxFolder, processFile)

	if err != nil {
		log.Println(err)
	}

	var zoom maptile.Zoom
	for zoom = 0; zoom <= maxZoom; zoom++ {
		fmt.Printf("Zoom: %d, Processing segments...\n", zoom)

		heatTiles := processSegments(segments, zoom)

		fmt.Printf("Zoom: %d, Searching maximum...\n", zoom)
		var max float64 = 0
		for _, tileHeat := range heatTiles {
			for x := 0; x < 256; x++ {
				for y := 0; y < 256; y++ {
					if max < tileHeat[x][y] {
						max = tileHeat[x][y]
					}
				}
			}
		}
		fmt.Printf("Zoom: %d, Maximum: %f\n", zoom, max)

		maxLog := math.Log(max * 10)
		for key, tileHeat := range heatTiles {
			tile := maptile.FromQuadkey(key, zoom)
			fmt.Printf("Zoom: %d, Normalizing tile %d, %d\n", zoom, tile.X, tile.Y)

			tileImage := heatTileToGraphicLog(maxLog, tileHeat)

			var dirName = fmt.Sprintf("D:/tiles/%d/%d", zoom, tile.X)
			os.MkdirAll(dirName, os.ModeDir)
			var name = fmt.Sprintf("D:/tiles/%d/%d/%d.png", zoom, tile.X, tile.Y)
			f, err := os.Create(name)
			if err != nil {
				panic(err)
			}

			if err = png.Encode(f, tileImage); err != nil {
				log.Printf("failed to encode: %v", err)
			}
			f.Close()
		}
	}
}

func processFile(path string, info os.FileInfo, err error) error {
	if info.IsDir() {
		return nil
	}

	ext := filepath.Ext(info.Name())

	if ext != ".gpx" {
		return nil
	}

	if err != nil {
		return err
	}

	gpxFile, err := gpx.ParseFile(path)

	if err != nil {
		return err
	}
	gpxPath, _ := filepath.Abs(path)

	fmt.Print("Reading file: ", gpxPath, "\n")

	const delta = 0.00002

	for _, track := range gpxFile.Tracks {
		for _, segment := range track.Segments {
			var prevPoint = Point{
				lat: randomize(segment.Points[0].Latitude, delta),
				lon: randomize(segment.Points[0].Longitude, delta),
			}
			for _, point := range segment.Points[1:] {
				to := Point{
					lat: randomize(point.Latitude, delta),
					lon: randomize(point.Longitude, delta),
				}
				segments = append(segments, &Segment{from: prevPoint, to: to})
				prevPoint = to
			}
		}
	}
	return nil
}

func randomize(value float64, delta float64) float64 {
	return value + delta*2*rand.Float64() - delta
}

func processSegments(segments []*Segment, zoom maptile.Zoom) map[uint64]*HeatTile {
	heatTiles := make(map[uint64]*HeatTile)
	for _, segment := range segments {
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
		var x1 = int(math.Floor((fromPointInt.X() - float64(fromTile.X)) * 256))
		var y1 = int(math.Floor((fromPointInt.Y() - float64(fromTile.Y)) * 256))

		var toPointInt = maptile.Fraction(toPoint, zoom)
		var x2 = int(math.Floor((toPointInt.X() - float64(fromTile.X)) * 256))
		var y2 = int(math.Floor((toPointInt.Y() - float64(fromTile.Y)) * 256))

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
			x1 = int(math.Floor((fromPointInt.X() - float64(toTile.X)) * 256))
			y1 = int(math.Floor((fromPointInt.Y() - float64(toTile.Y)) * 256))

			toPointInt = maptile.Fraction(toPoint, zoom)
			x2 = int(math.Floor((toPointInt.X() - float64(toTile.X)) * 256))
			y2 = int(math.Floor((toPointInt.Y() - float64(toTile.Y)) * 256))

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
	graphic := image.NewNRGBA(image.Rect(0, 0, 256, 256))
	for x := 0; x < 256; x++ {
		for y := 0; y < 256; y++ {
			heat := tile[x][y]
			if heat > 0 {
				pixLog := math.Log(heat * 10)
				normalized := pixLog / maxLog * 255
				intNormalized := uint8(normalized)

				pixColor := color.NRGBA{R: 255 - intNormalized, G: intNormalized, B: 0, A: intNormalized}
				graphic.SetNRGBA(x, y, pixColor)
			}
		}
	}
	return graphic
}

func (h *HeatTile) Set(x int, y int, c color.Color) {
	if x >= 0 && y >= 0 && x < 256 && y < 256 {
		h[x][y] += 1
	}
}

func (h *HeatTile) UnSet(x int, y int, c color.Color) {
	if x >= 0 && y >= 0 && x < 256 && y < 256 {
		h[x][y] -= 1
	}
}
