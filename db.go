package main

import (
	"database/sql"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/mattn/go-sqlite3"
	"time"
)

type Repository struct {
	db *sql.DB
}

const migrationsPath = "db/migrations"
const dbPath = "db/tracks.db"

func InitRepository() (*Repository, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	driver, err := sqlite3.WithInstance(db, &sqlite3.Config{})
	if err != nil {
		return nil, err
	}

	m, err := migrate.NewWithDatabaseInstance("file://"+migrationsPath, "sqlite3", driver)
	if err != nil {
		return nil, err
	}

	err = m.Up()
	if err != nil && err != migrate.ErrNoChange {
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
	return nil
}

func (repo *Repository) insertPoint(trackId int, position int, time time.Time, p *Point, ele float64) error {
	stm, err := repo.db.Prepare(
		"INSERT INTO track_points (track_id, position, lat, lon, time, ele) VALUES (?, ?, ?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer stm.Close()

	_, err = stm.Exec(trackId, position, p.lat, p.lon, time.Unix(), ele)
	if err != nil {
		return err
	}
	return nil
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
	} else if err == sql.ErrNoRows {
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

func (repo *Repository) segments() ([]*Segment, error) {
	segments := make([]*Segment, 0)
	rows, err := repo.db.Query(
		"SELECT tp1.lat, tp1.lon, tp2.lat, tp2.lon FROM track_points tp1 JOIN track_points tp2 ON tp1.track_id == tp2.track_id AND tp1.position + 1 = tp2.position")
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
	return segments, nil
}
