-- name: GetUser :one
SELECT id, email, status FROM app.users WHERE id = $1;

-- name: ListActiveUsers :many
SELECT id, email FROM app.users WHERE status = 'active' ORDER BY created_at DESC;

-- name: ListUserOrders :many
SELECT o.id, o.total, o.placed_at
FROM app.orders o
JOIN app.users u ON u.id = o.user_id
WHERE u.id = $1;

-- name: CreateOrder :one
INSERT INTO app.orders (user_id, total) VALUES ($1, $2) RETURNING id, placed_at;

-- name: GetProfile :one
SELECT user_id, bio, address FROM app.profiles WHERE user_id = @user_id;

-- name: DeleteUser :exec
DELETE FROM app.users WHERE id = $1;

-- name: SetUserStatus :execrows
UPDATE app.users SET status = $2 WHERE id = $1;
