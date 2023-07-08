package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strconv"
)

type App struct {
	repo      *Repository
	err       error
	segments  *Segments
	heatTiles *ZoomHeatTileSet

	tracksPath *string
	tilesPath  *string
	port       *int
	onlyServe  *bool
}

var re = regexp.MustCompile(`/tiles/(\d+)/(\d+)/(\d+).png`)

func (a App) execute() {
	if *a.tracksPath != "" && !*a.onlyServe {
		fmt.Printf("Importing tracks from: %s\n", *a.tracksPath)

		a.err = importTracks(*a.tracksPath, a.repo)
		a.checkError()
	}
	if *a.tilesPath != "" && !*a.onlyServe {
		fmt.Printf("Building tiles from database into: %s\n", *a.tilesPath)

		a.segments, a.err = a.repo.segments()
		a.checkError()

		a.err = buildTiles(*a.tilesPath, a.segments)
		a.checkError()
	}
	if *a.port != 0 {
		http.HandleFunc("/", a.getRoot)
		http.HandleFunc("/tiles/", a.getTile)
		fmt.Printf("Starting server on port %d\n", *a.port)
		a.err = http.ListenAndServe(fmt.Sprintf(":%d", *a.port), nil)
		a.checkError()
	}
}

func (a App) getRoot(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "public/index.html")
}

func (a App) getTile(w http.ResponseWriter, r *http.Request) {
	matches := re.FindStringSubmatch(r.URL.String())
	if len(matches) != 4 {
		w.WriteHeader(http.StatusNotFound)
	}
	zoom, err1 := strconv.Atoi(matches[1])
	x, err2 := strconv.Atoi(matches[2])
	y, err3 := strconv.Atoi(matches[3])
	if err1 != nil || err2 != nil || err3 != nil {
		w.WriteHeader(http.StatusNotFound)
	}
	filename := fmt.Sprintf("%s/%d/%d/%d.png", *a.tilesPath, zoom, x, y)

	if _, err := os.Stat(filename); errors.Is(err, os.ErrNotExist) {
		filename = fmt.Sprintf("%s/empty.png", *a.tilesPath)
	}

	fmt.Printf("IP: %s, Zoom: %d, x: %d, y: %d, file: %s\n", r.RemoteAddr, zoom, x, y, filename)
	http.ServeFile(w, r, filename)
}

func (a App) checkError() {
	if a.err != nil {
		panic(a.err)
	}
}
