-- name: SearchUsers :many
SELECT id, email, status FROM app.users
WHERE
      email = @email     -- @if
  AND id = ANY(@ids)     -- @if
-- @orderby created_at, email
;
