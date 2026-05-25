-- name: GetAuthor :one
SELECT id, name, bio FROM authors WHERE id = $1;

-- name: ListAuthors :many
SELECT id, name, bio FROM authors;

-- name: GetBook :one
SELECT id, author_id, title, published FROM books WHERE id = $1;

-- name: DeleteAuthor :exec
DELETE FROM authors WHERE id = $1;
