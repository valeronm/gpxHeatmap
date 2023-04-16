package main

import (
	"flag"
	"fmt"
)

func main() {
	repo, err := InitRepository()
	panicOnError(err)

	defer repo.close()

	flag.Parse()
	args := flag.Args()
	if len(args) == 1 {
		gpxFolder := args[0]
		fmt.Printf("Importing tracks from: %s\n", gpxFolder)

		err := importTracks(gpxFolder, repo)
		panicOnError(err)
	} else {
		fmt.Printf("Building tiles from database\n")

		segments, err := repo.segments()
		panicOnError(err)

		err = buildTiles(segments)
		panicOnError(err)
	}
}

func panicOnError(err error) {
	if err != nil {
		panic(err)
	}
}
