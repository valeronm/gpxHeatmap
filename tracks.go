package main

import (
	"fmt"
	"github.com/tkrajina/gpxgo/gpx"
	"math/rand"
	"os"
	"path/filepath"
)

const delta = 0 // 0.00002

type Point struct {
	lat float64
	lon float64
}

type Segment struct {
	from Point
	to   Point
}

type Context struct {
	repo     *Repository
	segments []*Segment
}

func importTracks(folder string, repo *Repository) ([]*Segment, error) {
	context := &Context{repo, make([]*Segment, 0)}
	err := filepath.Walk(folder, context.processFile)

	if err != nil {
		return nil, err
	} else {
		return context.segments, nil
	}
}

func (ctx *Context) processFile(path string, info os.FileInfo, err error) error {
	if err != nil {
		return err
	}

	if info.IsDir() {
		return nil
	}

	ext := filepath.Ext(info.Name())

	if ext != ".gpx" {
		return nil
	}

	gpxFile, err := gpx.ParseFile(path)

	if err != nil {
		return err
	}
	gpxPath, _ := filepath.Abs(path)

	fmt.Print("Reading file: ", gpxPath, "\n")
	firstTrack := gpxFile.Tracks[0]
	firstSegment := firstTrack.Segments[0]
	firstPoint := firstSegment.Points[0]

	lastTrack := gpxFile.Tracks[len(gpxFile.Tracks)-1]
	lastSegment := lastTrack.Segments[len(lastTrack.Segments)-1]
	lastPoint := lastSegment.Points[len(lastSegment.Points)-1]

	err = ctx.repo.beginTransaction()
	if err != nil {
		return err
	}
	trackId, err := ctx.repo.upsertTrack(filepath.Base(gpxPath), firstPoint.Timestamp, lastPoint.Timestamp)
	if err != nil {
		return err
	}
	err = ctx.repo.clearTrack(trackId)
	if err != nil {
		return err
	}

	position := 0
	for _, track := range gpxFile.Tracks {
		for _, segment := range track.Segments {
			firstPoint := segment.Points[0]
			var from = Point{
				lat: randomize(firstPoint.Latitude, delta),
				lon: randomize(firstPoint.Longitude, delta),
			}
			err := ctx.repo.insertPoint(trackId, position, firstPoint.Timestamp, &from, firstPoint.Elevation.Value())
			if err != nil {
				return err
			}
			position += 1
			for _, point := range segment.Points[1:] {
				to := Point{
					lat: randomize(point.Latitude, delta),
					lon: randomize(point.Longitude, delta),
				}
				err := ctx.repo.insertPoint(trackId, position, point.Timestamp, &to, point.Elevation.Value())
				if err != nil {
					return err
				}
				ctx.segments = append(ctx.segments, &Segment{from, to})
				from = to
				position += 1
			}
		}
	}
	err = ctx.repo.commit()
	if err != nil {
		return err
	}
	return nil
}

func randomize(value float64, delta float64) float64 {
	return value + delta*2*rand.Float64() - delta
}
