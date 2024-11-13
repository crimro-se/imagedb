-- uses 'rowid' innate primary key.
CREATE TABLE IF NOT EXISTS images (
  relative_path TEXT UNIQUE NOT NULL,
  sub_path TEXT,
  tags  TEXT,
  aesthetic REAL
);
CREATE INDEX IF NOT EXISTS aesthetic_idx ON images(aesthetic);
CREATE INDEX IF NOT EXISTS path_idx ON images(relative_path, sub_path);
-- TODO: CreatedAt


-- uses 'rowid' innate primary key. 
CREATE VIRTUAL TABLE IF NOT EXISTS embeddings USING vec0 (
    embedding FLOAT[768]
);

/* queries reference

CREATE
  CreateUpdateImage
  CreateUpdateEmbedding

READ
  ReadImages

SEARCH
  MatchEmbeddings
  MatchImages

UPDATE
  CreateUpdateImage
  CreateUpdateEmbedding

DELETE


*/