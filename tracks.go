package main

import (
	"fmt"
	"github.com/tkrajina/gpxgo/gpx"
	"math"
	"os"
	"path/filepath"
	"time"
)

type Point struct {
	lat float64
	lon float64
}

type Segment struct {
	from Point
	to   Point
}

type Segments []*Segment

type Context struct {
	repo *Repository
}

func importTracks(folder string, repo *Repository) error {
	context := &Context{repo}
	err := filepath.Walk(folder, context.processFile)

	if err != nil {
		return err
	}
	return nil
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

	gpxPath, _ := filepath.Abs(path)
	trackId, err := ctx.repo.findTrack(filepath.Base(gpxPath))
	if err != nil {
		return err
	}

	if trackId != 0 {
		fmt.Print("Skipping file: ", gpxPath, "\n")
		return nil
	}

	fmt.Print("Reading file: ", gpxPath)
	gpxFile, err := gpx.ParseFile(path)
	if err != nil {
		return err
	}

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

	trackId, err = ctx.repo.insertTrack(filepath.Base(gpxPath), firstPoint.Timestamp, lastPoint.Timestamp)
	if err != nil {
		ctx.repo.rollback()
		return err
	}
	err = ctx.repo.clearTrack(trackId)
	if err != nil {
		ctx.repo.rollback()
		return err
	}

	totalLength := 0.0

	var prevTimestamp *time.Time = nil
	var prevGeoPoint *Point = nil
	prevTrackPointId := 0
	position := 0
	for _, track := range gpxFile.Tracks {
		for _, segment := range track.Segments {
			for _, point := range segment.Points {
				geoPoint := Point{
					lat: point.Latitude,
					lon: point.Longitude,
				}
				timestamp := point.Timestamp
				pointId, err := ctx.repo.insertPoint(trackId, position, timestamp, &geoPoint, point.Elevation.Value())
				if err != nil {
					ctx.repo.rollback()
					return err
				}
				if prevGeoPoint != nil && prevTrackPointId > 0 {
					length := gpx.HaversineDistance(prevGeoPoint.lat, prevGeoPoint.lon, geoPoint.lat, geoPoint.lon)
					totalLength += length
					seconds := timestamp.Sub(*prevTimestamp).Seconds()
					if seconds == 0 {
						seconds = 1
					}
					velocity := length / seconds
					_, err := ctx.repo.insertSegment(trackId, prevTrackPointId, pointId, length, velocity)
					if err != nil {
						ctx.repo.rollback()
						return err
					}
				}
				prevTimestamp = &timestamp
				prevGeoPoint = &geoPoint
				prevTrackPointId = pointId
				position += 1
			}
		}
	}

	fmt.Print(" - ", math.Round(totalLength/100.0)/10.0, " km\n")

	err = ctx.repo.commit()
	if err != nil {
		return err
	}
	return nil
}
