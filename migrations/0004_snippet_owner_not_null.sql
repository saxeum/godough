-- Contract. Run this as a second deploy, after the binary that always writes
-- owner_id is the only one serving. Keeping it separate is what makes the
-- expand window safe: the migration Job runs before the Deployment.
ALTER TABLE snippets ALTER COLUMN owner_id SET NOT NULL;
