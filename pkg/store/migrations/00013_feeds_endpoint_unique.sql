-- +migrate up
-- ORCH-4 (ADR 0012): automatic, collision-free multicast endpoint allocation.
-- Each orchestrated Firefly instance emits on its own multicast group/port; two
-- feeds sharing a (group, port) would cross-talk on the wire — a tenant could see
-- another's datagrams at the network layer, before scoped fan-out even applies.
-- This UNIQUE constraint makes a duplicate endpoint impossible and is the
-- race-safe backstop for the allocator (it allocates the next free endpoint and
-- relies on this constraint to settle concurrent inserts).
--
-- Assumes no existing duplicate (group, port) rows; true for current single-feed
-- catalogues. Manual entries that collide are rejected at the API (409); the
-- allocator skips taken endpoints.
ALTER TABLE feeds ADD CONSTRAINT feeds_endpoint_unique UNIQUE (multicast_group, port);

-- +migrate down
ALTER TABLE feeds DROP CONSTRAINT IF EXISTS feeds_endpoint_unique;
