CREATE TABLE feeds (kind text NOT NULL, id bytea NOT NULL, parent_kind text, parent_id bytea, PRIMARY KEY(kind,id));
CREATE TABLE purged_feeds (kind text NOT NULL, id bytea NOT NULL);
CREATE TABLE device_owners(device bytea PRIMARY KEY, owner bytea NOT NULL, seen_at timestamptz NOT NULL);
CREATE TABLE device_delivery(device bytea PRIMARY KEY, position bigint NOT NULL);
CREATE TABLE feed_snapshots(kind text NOT NULL, id bytea NOT NULL, position bigint NOT NULL, type bigint NOT NULL, payload bytea NOT NULL, PRIMARY KEY(kind,id));
CREATE TABLE feed_items(kind text NOT NULL, id bytea NOT NULL, position bigint NOT NULL, expires timestamptz NOT NULL);
