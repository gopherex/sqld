-- name: SearchUsers :many
SELECT id, email, status FROM app.users
WHERE
      email = @email?
  AND id = ANY(@ids)
-- @orderby created_at, email
;
