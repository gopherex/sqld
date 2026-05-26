CREATE TYPE account_status AS ENUM ('active','suspended','closed');

CREATE TABLE accounts (
  id          bigserial PRIMARY KEY,
  email       text NOT NULL,
  status      account_status NOT NULL,
  is_verified boolean NOT NULL DEFAULT false,
  created_at  timestamptz NOT NULL DEFAULT now(),
  nickname    text
);
