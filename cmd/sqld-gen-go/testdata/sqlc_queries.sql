-- name: FindName :many
SELECT id FROM profiles WHERE name = sqlc.narg('name') ORDER BY id;

-- name: DynamicFind :many
SELECT id FROM profiles WHERE name = sqlc.narg('name') AND id = @id? ORDER BY id;

-- name: FindIDs :many
SELECT id FROM profiles WHERE id = ANY(sqlc.narg('ids')::bigint[]) ORDER BY id;

-- name: RepeatName :one
SELECT sqlc.narg('name')::text IS NULL AS missing,
       COALESCE(sqlc.arg(name)::text, @name::text, 'fallback') AS value;

-- name: UpdateName :execrows
UPDATE profiles SET name = COALESCE(sqlc.narg('name'), name) WHERE id = sqlc.arg('id');

-- name: ClearBio :exec
UPDATE profiles SET bio = sqlc.narg('bio') WHERE id = @id;

-- name: CreateProfile :one
WITH inserted AS (
    INSERT INTO profiles(id, name, bio)
    VALUES (sqlc.arg('id'), COALESCE(sqlc.narg('name')::text, 'default'), sqlc.narg('bio'))
    RETURNING id, name, bio
)
SELECT id, name, bio FROM inserted;

-- name: MixedPositional :one
SELECT $1::bigint + sqlc.arg('amount')::bigint AS value;
