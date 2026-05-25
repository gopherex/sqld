-- name: SearchUsers :many
SELECT id, email, status FROM app.users
WHERE true
/*@if name*/ AND email = $1 /*@endif*/
/*@slice ids*/ AND id = ANY($2) /*@endif*/
/*@orderby allow=created_at,email*/;
