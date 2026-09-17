\set ON_ERROR_STOP on
BEGIN;
INSERT INTO users(id,email,password_hash) VALUES('explain-user','explain@pulseroute.test','not-a-login');
INSERT INTO projects(id,owner_id,name) VALUES('explain-project','explain-user','Synthetic query plan');
INSERT INTO events(id,project_id,event_id,type,payload,payload_hash,created_at)
SELECT 'explain-'||n,'explain-project','explain-'||n,
CASE WHEN n%100=0 THEN 'invoice.failed' ELSE 'order.created' END,
'{"synthetic":true}'::jsonb,'synthetic',now()-n*interval '1 second'
FROM generate_series(1,20000) n;
ANALYZE events;
\echo With composite type/time index
EXPLAIN (ANALYZE,BUFFERS) SELECT id,type,created_at FROM events
WHERE project_id='explain-project' AND type='invoice.failed'
ORDER BY created_at DESC,id LIMIT 25;
DROP INDEX events_type_time;
\echo Without composite type/time index
EXPLAIN (ANALYZE,BUFFERS) SELECT id,type,created_at FROM events
WHERE project_id='explain-project' AND type='invoice.failed'
ORDER BY created_at DESC,id LIMIT 25;
ROLLBACK;
