-- name: CreateFeed :one
WITH buried AS (SELECT 1 FROM purged_feeds WHERE kind=@kind AND id=@id), made AS (
 INSERT INTO feeds(kind,id,parent_kind,parent_id)
 SELECT @kind,@id,@parent_kind,@parent_id WHERE NOT EXISTS(SELECT 1 FROM buried)
 ON CONFLICT DO NOTHING)
SELECT EXISTS(SELECT 1 FROM buried) AS purged;
-- name: BindDevice :one
WITH bound AS (INSERT INTO device_owners(device,owner,seen_at) VALUES(@device,@owner,now())
 ON CONFLICT(device) DO UPDATE SET seen_at=now() WHERE device_owners.owner=EXCLUDED.owner RETURNING owner)
SELECT owner FROM bound UNION ALL SELECT owner FROM device_owners WHERE device=@device LIMIT 1;
-- name: StoreSnapshot :exec
WITH stored AS (INSERT INTO feed_snapshots(kind,id,position,type,payload) VALUES(@kind,@id,@position::bigint,@type::bigint,@payload::bytea)
 ON CONFLICT(kind,id) DO UPDATE SET position=EXCLUDED.position,type=EXCLUDED.type,payload=EXCLUDED.payload
 WHERE feed_snapshots.position<=EXCLUDED.position RETURNING position)
UPDATE feed_items SET expires=now() WHERE kind=@kind AND id=@id AND position<=(SELECT position FROM stored) AND expires='infinity';
-- name: SweepDevices :one
WITH dead AS (DELETE FROM device_owners WHERE device IN (
 SELECT device FROM device_owners WHERE seen_at<@dead_before ORDER BY seen_at LIMIT @limit FOR UPDATE SKIP LOCKED) RETURNING device),
 delivery AS (DELETE FROM device_delivery WHERE device IN (SELECT device FROM dead))
SELECT count(*)::int AS removed FROM dead;
-- name: ChangeParent :one
WITH changed(parent_kind,parent_id) AS (UPDATE feeds f SET parent_kind=@parent_kind,parent_id=@parent_id WHERE f.kind=@kind AND f.id=@id RETURNING f.parent_kind,f.parent_id)
SELECT * FROM changed;
-- name: SnapshotType :one
WITH stored AS (INSERT INTO feed_snapshots(kind,id,position,type,payload) VALUES(@kind,@id,1,@type,'') RETURNING type)
SELECT type FROM stored;
