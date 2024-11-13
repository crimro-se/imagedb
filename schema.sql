-- uses 'rowid' innate primary key.
CREATE TABLE IF NOT EXISTS basedir (
  directory TEXT                  -- a location the user has decided to index
);
CREATE UNIQUE INDEX IF NOT EXISTS basedir_path_idx ON basedir(directory);

-- uses 'rowid' innate primary key.
CREATE TABLE IF NOT EXISTS images (
  basedir_id INTEGER NOT NULL,    -- our path is relative from this directory
  parent_path TEXT NOT NULL,      -- may still legitimately be an 'empty string'
                                  -- parent path is the path of the folder OR archive containing the image
  sub_path TEXT NOT NULL,                  -- either the filename of an image, or the path to it within an archive
  tags  TEXT,                     -- UNIMPLEMENTED
  aesthetic REAL,                 -- an AI's oppinion on the attractiveness of this image, between 0 and 10.
  width INTEGER NOT NULL,         -- basic attributes about the image.
  height INTEGER NOT NULL,
  filesize INTEGER NOT NULL,
  FOREIGN KEY (basedir_id) REFERENCES basedir(rowid)
);
CREATE INDEX IF NOT EXISTS images_basedir_id_idx ON images(basedir_id);
CREATE INDEX IF NOT EXISTS images_aesthetic_idx ON images(aesthetic);
CREATE UNIQUE INDEX IF NOT EXISTS images_path_idx ON images(parent_path, sub_path);
-- TODO: CreatedAt?


-- uses 'rowid' innate primary key.
-- can only have 1:1 relationship with images, as rowid is both pk and foreign key.
CREATE VIRTUAL TABLE IF NOT EXISTS embeddings USING vec0 (
    embedding float[768]
);

/* queries reference (database.go)

CREATE
  CreateUpdateImage
  CreateUpdateEmbedding

READ
  ReadImages
  MatchEmbeddings
  MatchImagesByPath

UPDATE
  CreateUpdateImage
  CreateUpdateEmbedding

DELETE


*/