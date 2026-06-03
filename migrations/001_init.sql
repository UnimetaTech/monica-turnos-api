CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS young_researchers (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name TEXT NOT NULL,
  email TEXT,
  active BOOLEAN NOT NULL DEFAULT TRUE,
  color TEXT NOT NULL DEFAULT '#2563eb',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS events (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  google_calendar_event_id TEXT UNIQUE,
  title TEXT NOT NULL,
  start_at TIMESTAMPTZ NOT NULL,
  end_at TIMESTAMPTZ NOT NULL,
  location TEXT NOT NULL DEFAULT '',
  modality TEXT NOT NULL DEFAULT 'presencial',
  status TEXT NOT NULL DEFAULT 'scheduled',
  required_people INT NOT NULL DEFAULT 0,
  source TEXT NOT NULL DEFAULT 'PANEL',
  description TEXT,
  calendar_synced_at TIMESTAMPTZ,
  created_by TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS assignments (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
  young_researcher_id UUID NOT NULL REFERENCES young_researchers(id) ON DELETE CASCADE,
  status TEXT NOT NULL DEFAULT 'assigned',
  assigned_by TEXT,
  source TEXT NOT NULL DEFAULT 'PANEL',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_active_assignment_per_event_researcher
ON assignments(event_id, young_researcher_id)
WHERE status IN ('assigned', 'confirmed');

CREATE TABLE IF NOT EXISTS assignment_history (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  assignment_id UUID,
  event_id UUID,
  young_researcher_id UUID,
  action TEXT NOT NULL,
  old_value JSONB,
  new_value JSONB,
  changed_by TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO young_researchers (name, email, color)
VALUES
  ('Sebastian', 'sebastian@unimeta.edu.co', '#2563eb'),
  ('Duvan', 'duvan@unimeta.edu.co', '#16a34a'),
  ('Montillo', 'montillo@unimeta.edu.co', '#9333ea'),
  ('Michael', 'michael@unimeta.edu.co', '#ea580c'),
  ('Jonathan', 'jonathan@unimeta.edu.co', '#0891b2'),
  ('Guzman', 'guzman@unimeta.edu.co', '#be123c')
ON CONFLICT DO NOTHING;
