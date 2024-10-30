-- use rowid innate indexes.
CREATE TABLE IF NOT EXISTS images (
  relative_path TEXT UNIQUE NOT NULL,
  sub_path TEXT,
  tags  TEXT
);
-- TODO: CreatedAt

CREATE VIRTUAL TABLE IF NOT EXISTS embeddings USING vec0 (
    embedding FLOAT[8]
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