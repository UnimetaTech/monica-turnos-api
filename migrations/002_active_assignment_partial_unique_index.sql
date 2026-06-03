ALTER TABLE assignments
DROP CONSTRAINT IF EXISTS assignments_event_id_young_researcher_id_key;

CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_active_assignment_per_event_researcher
ON assignments(event_id, young_researcher_id)
WHERE status IN ('assigned', 'confirmed');
