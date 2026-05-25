CREATE SCHEMA app;
CREATE SCHEMA audit;

CREATE TYPE app.user_status AS ENUM ('active', 'inactive', 'banned');
CREATE DOMAIN app.email AS text NOT NULL CHECK (VALUE ~ '@');
CREATE TYPE app.address AS (street text, city text, zip text);

CREATE SEQUENCE app.order_number_seq;

CREATE TABLE app.users (
  id          bigserial PRIMARY KEY,
  email       app.email NOT NULL UNIQUE,
  status      app.user_status NOT NULL DEFAULT 'active',
  manager_id  bigint REFERENCES app.users(id) ON DELETE SET NULL,
  created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE app.profiles (
  user_id bigint PRIMARY KEY REFERENCES app.users(id) ON DELETE CASCADE,
  bio     text,
  address app.address
);

CREATE TABLE app.orders (
  id        bigint PRIMARY KEY DEFAULT nextval('app.order_number_seq'),
  user_id   bigint NOT NULL REFERENCES app.users(id) ON DELETE CASCADE,
  total     numeric(12,2) NOT NULL CHECK (total >= 0),
  placed_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE app.roles (
  id   bigserial PRIMARY KEY,
  name text NOT NULL UNIQUE
);

CREATE TABLE app.user_roles (
  user_id bigint NOT NULL REFERENCES app.users(id) ON DELETE CASCADE,
  role_id bigint NOT NULL REFERENCES app.roles(id) ON DELETE CASCADE,
  PRIMARY KEY (user_id, role_id)
);

CREATE INDEX idx_orders_user ON app.orders (user_id);
CREATE UNIQUE INDEX idx_users_email_lower ON app.users (lower(email));
CREATE INDEX idx_orders_recent ON app.orders (placed_at) WHERE total > 100;

CREATE FUNCTION app.touch_updated() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RETURN NEW; END $$;

CREATE TRIGGER users_touch BEFORE UPDATE ON app.users
  FOR EACH ROW EXECUTE FUNCTION app.touch_updated();

CREATE VIEW app.active_users AS
  SELECT id, email FROM app.users WHERE status = 'active';

CREATE MATERIALIZED VIEW app.user_order_counts AS
  SELECT u.id, count(o.id) AS order_count
  FROM app.users u LEFT JOIN app.orders o ON o.user_id = u.id
  GROUP BY u.id;
