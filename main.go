package main

import (
	"flag"
)

func main() {
	app := App{}
	app.repo, app.err = InitRepository()
	app.checkError()

	defer app.repo.close()

	app.tracksPath = flag.String("I", "", "Folder with GPX tracks for import")
	app.tilesPath = flag.String("O", "", "Folder for storing built tiles")
	app.port = flag.Int("P", 0, "Port for starting server")
	app.onlyServe = flag.Bool("serve-only", false, "Port for starting server")
	flag.Parse()

	app.execute()
}
