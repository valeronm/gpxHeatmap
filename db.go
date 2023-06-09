package main

import (
	"database/sql"
	"errors"
	_ "modernc.org/sqlite"
	"time"
)

type Repository struct {
	db *sql.DB
}

const dbPath = "db/tracks.db"

const migration = `
CREATE TABLE IF NOT EXISTS tracks
(
    id         INTEGER PRIMARY KEY,
    file_name  TEXT NOT NULL,
    start_time INT  NOT NULL,
    end_time   INT  NOT NULL
);

CREATE TABLE  IF NOT EXISTS track_points
(
    id       INTEGER PRIMARY KEY,
    track_id INTEGER NOT NULL,
    position INTEGER NOT NULL,
    time     INT     NOT NULL,
    lat      REAL    NOT NULL,
    lon      REAL    NOT NULL,
    ele      READ    NOT NULL,

    FOREIGN KEY (track_id)
        REFERENCES tracks (id)
        ON DELETE RESTRICT
);

CREATE TABLE  IF NOT EXISTS track_segments
(
    id       INTEGER PRIMARY KEY,

    track_id INTEGER NOT NULL,
    start_point_id INTEGER NOT NULL,
    end_point_id INTEGER NOT NULL,

    length REAL NOT NULL,
    velocity REAL NOT NULL,

    FOREIGN KEY (start_point_id)
        REFERENCES track_points (id)
        ON DELETE RESTRICT
    FOREIGN KEY (end_point_id)
        REFERENCES track_points (id)
        ON DELETE RESTRICT
);
CREATE INDEX IF NOT EXISTS idx_lat_lon ON track_points (lat, lon);
`

func InitRepository() (*Repository, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(migration)
	if err != nil {
		return nil, err
	}

	return &Repository{db}, nil
}

func (repo *Repository) close() error {
	err := repo.db.Close()
	if err != nil {
		return err
	} else {
		return nil
	}
}

func (repo *Repository) clearTrack(trackId int) error {
	stm, err := repo.db.Prepare("DELETE FROM track_points WHERE track_id = ?")
	if err != nil {
		return err
	}
	defer stm.Close()

	_, err = stm.Exec(trackId)
	if err != nil {
		return err
	}

	stm, err = repo.db.Prepare("DELETE FROM track_segments WHERE track_id = ?")
	if err != nil {
		return err
	}
	defer stm.Close()

	_, err = stm.Exec(trackId)
	if err != nil {
		return err
	}
	return nil
}

func (repo *Repository) insertPoint(trackId int, position int, time time.Time, p *Point, ele float64) (int, error) {
	stm, err := repo.db.Prepare(
		"INSERT INTO track_points (track_id, position, lat, lon, time, ele) VALUES (?, ?, ?, ?, ?, ?)  RETURNING id")
	if err != nil {
		return 0, err
	}
	defer stm.Close()

	row := stm.QueryRow(trackId, position, p.lat, p.lon, time.Unix(), ele)
	if row.Err() != nil {
		return 0, row.Err()
	}

	id := 0
	err = row.Scan(&id)
	if err == nil {
		return id, nil
	} else {
		return 0, err
	}
}

func (repo *Repository) insertSegment(trackId int, startPointId int, endPointId int, length float64, velocity float64) (int, error) {
	stm, err := repo.db.Prepare(
		"INSERT INTO track_segments (track_id, start_point_id, end_point_id, length, velocity) VALUES (?, ?, ?, ?, ?)  RETURNING id")
	if err != nil {
		return 0, err
	}
	defer stm.Close()

	row := stm.QueryRow(trackId, startPointId, endPointId, length, velocity)
	if row.Err() != nil {
		return 0, row.Err()
	}

	id := 0
	err = row.Scan(&id)
	if err == nil {
		return id, nil
	} else {
		return 0, err
	}
}

func (repo *Repository) findTrack(filename string) (int, error) {
	stm, err := repo.db.Prepare("SELECT id FROM tracks WHERE file_name = ?")
	if err != nil {
		return 0, err
	}
	defer stm.Close()

	row := stm.QueryRow(filename)
	if row.Err() != nil {
		return 0, row.Err()
	}

	id := 0
	err = row.Scan(&id)
	if err == nil {
		return id, nil
	} else if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	} else {
		return 0, err
	}
}

func (repo *Repository) insertTrack(filename string, start time.Time, end time.Time) (int, error) {
	stm, err := repo.db.Prepare(
		"INSERT INTO tracks (file_name, start_time, end_time) values (?, ?, ?) RETURNING id")
	if err != nil {
		return 0, err
	}
	defer stm.Close()

	row := stm.QueryRow(filename, start.Unix(), end.Unix())
	if row.Err() != nil {
		return 0, row.Err()
	}

	id := 0
	err = row.Scan(&id)
	if err == nil {
		return id, nil
	} else {
		return 0, err
	}
}

func (repo *Repository) beginTransaction() error {
	return repo.execute("BEGIN TRANSACTION")
}

func (repo *Repository) commit() error {
	return repo.execute("COMMIT")
}

func (repo *Repository) rollback() error {
	return repo.execute("ROLLBACK")
}

func (repo *Repository) execute(query string) error {
	_, err := repo.db.Exec(query)
	if err != nil {
		return err
	}
	return nil
}

func (repo *Repository) segments() (*Segments, error) {
	segments := make(Segments, 0)
	rows, err := repo.db.Query(
		`
SELECT 
    tp1.lat, tp1.lon, tp2.lat, tp2.lon
FROM track_segments ts
JOIN track_points tp1 ON ts.start_point_id = tp1.id
JOIN track_points tp2 ON ts.end_point_id = tp2.id
WHERE length > 0 and velocity >=2 and velocity <= 50;
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lat1, lat2, lon1, lon2 float64

	for rows.Next() {
		err = rows.Scan(&lat1, &lon1, &lat2, &lon2)
		if err != nil {
			return nil, err
		}
		from := Point{lat1, lon1}
		to := Point{lat2, lon2}
		segments = append(segments, &Segment{from, to})
	}
	return &segments, nil
}

func (repo *Repository) segmentsForRange(min *Point, max *Point) (*Segments, error) {
	segments := make(Segments, 0)

	rows, err := repo.db.Query(
		`
				SELECT 
					stp.lat, stp.lon, etp.lat, etp.lon
				FROM track_segments ts
				JOIN track_points stp ON ts.start_point_id = stp.id
				JOIN track_points etp ON ts.end_point_id = etp.id
				WHERE length > 0 and velocity >=2 and velocity <= 50
				AND (stp.lat >= ? OR etp.lat >= ?) AND (stp.lat <= ? OR etp.lat <= ?) 
				AND (stp.lon >= ? OR etp.lon >= ?) AND (stp.lon <= ? OR etp.lon <= ?);
				`,
		min.lat, min.lat, max.lat, max.lat, min.lon, min.lon, max.lon, max.lon)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lat1, lat2, lon1, lon2 float64

	for rows.Next() {
		err = rows.Scan(&lat1, &lon1, &lat2, &lon2)
		if err != nil {
			return nil, err
		}
		from := Point{lat1, lon1}
		to := Point{lat2, lon2}
		segments = append(segments, &Segment{from, to})
	}
	return &segments, nil
}
