-- Expand. The previous binary knows nothing about owner_id, so it keeps writing
-- rows without one while this migration is applied before the new Deployment.
-- Those rows are deliberately invisible to the new binary until the contract
-- migration removes the nullable window.
--
-- The seed rows have no honest owner. Delete them rather than invent one; the
-- tests create what they assert on, and the e2e script creates its own rows.
DELETE FROM snippets;

ALTER TABLE snippets
    ADD COLUMN owner_id bigint REFERENCES users(id) ON DELETE CASCADE;

-- The old index has no reader once every query filters by owner. The new one is
-- the list query's sort key: owner, then newest first with id as the tiebreak.
DROP INDEX idx_snippets_created;
CREATE INDEX idx_snippets_owner_created ON snippets (owner_id, created DESC, id DESC);
