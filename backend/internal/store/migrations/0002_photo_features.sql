ALTER TABLE photos ADD COLUMN favorite INTEGER NOT NULL DEFAULT 0;
ALTER TABLE photos ADD COLUMN deleted_at TEXT;

CREATE TABLE IF NOT EXISTS photo_sources (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    owner_id   INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    path       TEXT NOT NULL,
    created_at TEXT NOT NULL,
    UNIQUE (owner_id, path)
);
CREATE INDEX IF NOT EXISTS idx_photo_sources_owner ON photo_sources(owner_id);
