CREATE TABLE tracks
(
    id         INTEGER PRIMARY KEY,

    file_name  TEXT NOT NULL,
    start_time INT  NOT NULL,
    end_time   INT  NOT NULL
);

CREATE TABLE track_points
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