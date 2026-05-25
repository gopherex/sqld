CREATE TABLE authors (
  id   bigserial PRIMARY KEY,
  name text NOT NULL,
  bio  text
);

CREATE TABLE books (
  id        bigserial PRIMARY KEY,
  author_id bigint NOT NULL REFERENCES authors(id) ON DELETE CASCADE,
  title     text NOT NULL,
  published date
);
