-- PrivacyOS database schema (SQLite)
-- Timestamps are stored as RFC3339 UTC strings so they sort lexicographically.

PRAGMA journal_mode = WAL;
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS users (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    username        TEXT    NOT NULL UNIQUE,
    display_name    TEXT    NOT NULL DEFAULT '',
    password_hash   TEXT    NOT NULL,
    role            TEXT    NOT NULL DEFAULT 'user' CHECK (role IN ('admin', 'user')),
    status          TEXT    NOT NULL DEFAULT 'active'
                    CHECK (status IN ('active', 'disabled', 'pending_deletion')),
    quota_bytes     INTEGER NOT NULL DEFAULT 0,
    permissions     TEXT    NOT NULL DEFAULT '{}',
    preferences     TEXT    NOT NULL DEFAULT '{}',
    totp_secret     TEXT    NOT NULL DEFAULT '',
    totp_enabled    INTEGER NOT NULL DEFAULT 0,
    locked_until    TEXT,
    created_at      TEXT    NOT NULL,
    updated_at      TEXT    NOT NULL,
    last_login_at   TEXT,
    last_login_ip   TEXT    NOT NULL DEFAULT '',
    delete_after    TEXT
);
CREATE INDEX IF NOT EXISTS idx_users_status ON users(status);

CREATE TABLE IF NOT EXISTS sessions (
    id          TEXT    PRIMARY KEY,
    user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    ip          TEXT    NOT NULL DEFAULT '',
    user_agent  TEXT    NOT NULL DEFAULT '',
    created_at  TEXT    NOT NULL,
    last_seen   TEXT    NOT NULL,
    expires_at  TEXT    NOT NULL,
    revoked_at  TEXT
);
CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);

CREATE TABLE IF NOT EXISTS login_attempts (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    username    TEXT    NOT NULL,
    ip          TEXT    NOT NULL DEFAULT '',
    success     INTEGER NOT NULL DEFAULT 0,
    reason      TEXT    NOT NULL DEFAULT '',
    created_at  TEXT    NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_login_attempts_user ON login_attempts(username, created_at);

CREATE TABLE IF NOT EXISTS audit_logs (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at  TEXT    NOT NULL,
    user_id     INTEGER,
    username    TEXT    NOT NULL DEFAULT '',
    category    TEXT    NOT NULL,
    action      TEXT    NOT NULL,
    target      TEXT    NOT NULL DEFAULT '',
    detail      TEXT    NOT NULL DEFAULT '',
    ip          TEXT    NOT NULL DEFAULT '',
    result      TEXT    NOT NULL DEFAULT 'ok'
);
CREATE INDEX IF NOT EXISTS idx_audit_created  ON audit_logs(created_at);
CREATE INDEX IF NOT EXISTS idx_audit_category ON audit_logs(category, created_at);
CREATE INDEX IF NOT EXISTS idx_audit_user     ON audit_logs(username, created_at);

CREATE TABLE IF NOT EXISTS settings (
    key         TEXT PRIMARY KEY,
    value       TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS shares (
    id              TEXT    PRIMARY KEY,
    token           TEXT    NOT NULL UNIQUE,
    name            TEXT    NOT NULL,
    path            TEXT    NOT NULL,
    owner_id        INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind            TEXT    NOT NULL DEFAULT 'file',
    password_hash   TEXT    NOT NULL DEFAULT '',
    expires_at      TEXT,
    download_only   INTEGER NOT NULL DEFAULT 1,
    max_downloads   INTEGER NOT NULL DEFAULT 0,
    downloads       INTEGER NOT NULL DEFAULT 0,
    size_bytes      INTEGER NOT NULL DEFAULT 0,
    created_at      TEXT    NOT NULL,
    last_access_at  TEXT
);
CREATE INDEX IF NOT EXISTS idx_shares_owner ON shares(owner_id);

CREATE TABLE IF NOT EXISTS trash_items (
    id              TEXT    PRIMARY KEY,
    owner_id        INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    original_path   TEXT    NOT NULL,
    trash_path      TEXT    NOT NULL,
    name            TEXT    NOT NULL,
    is_dir          INTEGER NOT NULL DEFAULT 0,
    size_bytes      INTEGER NOT NULL DEFAULT 0,
    deleted_at      TEXT    NOT NULL,
    expires_at      TEXT
);
CREATE INDEX IF NOT EXISTS idx_trash_owner ON trash_items(owner_id);

CREATE TABLE IF NOT EXISTS storage_pools (
    id          TEXT    PRIMARY KEY,
    name        TEXT    NOT NULL UNIQUE,
    level       TEXT    NOT NULL DEFAULT 'single',
    fs          TEXT    NOT NULL DEFAULT 'ext4',
    devices     TEXT    NOT NULL DEFAULT '[]',
    mountpoint  TEXT    NOT NULL DEFAULT '',
    status      TEXT    NOT NULL DEFAULT 'inactive',
    size_bytes  INTEGER NOT NULL DEFAULT 0,
    created_at  TEXT    NOT NULL,
    updated_at  TEXT    NOT NULL
);

CREATE TABLE IF NOT EXISTS smart_tests (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    device      TEXT    NOT NULL,
    kind        TEXT    NOT NULL DEFAULT 'short',
    status      TEXT    NOT NULL DEFAULT 'completed',
    summary     TEXT    NOT NULL DEFAULT '',
    raw         TEXT    NOT NULL DEFAULT '',
    created_at  TEXT    NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_smart_device ON smart_tests(device, created_at);

CREATE TABLE IF NOT EXISTS photos (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    owner_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    path        TEXT    NOT NULL,
    file_name   TEXT    NOT NULL DEFAULT '',
    size_bytes  INTEGER NOT NULL DEFAULT 0,
    width       INTEGER NOT NULL DEFAULT 0,
    height      INTEGER NOT NULL DEFAULT 0,
    taken_at    TEXT,
    camera      TEXT    NOT NULL DEFAULT '',
    lens        TEXT    NOT NULL DEFAULT '',
    gps_lat     REAL,
    gps_lng     REAL,
    orientation INTEGER NOT NULL DEFAULT 1,
    thumb_path  TEXT    NOT NULL DEFAULT '',
    sha256      TEXT    NOT NULL DEFAULT '',
    created_at  TEXT    NOT NULL,
    UNIQUE (owner_id, path)
);
CREATE INDEX IF NOT EXISTS idx_photos_owner_taken ON photos(owner_id, taken_at);

CREATE TABLE IF NOT EXISTS albums (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    owner_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name        TEXT    NOT NULL,
    description TEXT    NOT NULL DEFAULT '',
    cover_photo INTEGER,
    created_at  TEXT    NOT NULL,
    UNIQUE (owner_id, name)
);

CREATE TABLE IF NOT EXISTS album_photos (
    album_id    INTEGER NOT NULL REFERENCES albums(id) ON DELETE CASCADE,
    photo_id    INTEGER NOT NULL REFERENCES photos(id) ON DELETE CASCADE,
    added_at    TEXT    NOT NULL,
    PRIMARY KEY (album_id, photo_id)
);

CREATE TABLE IF NOT EXISTS movies (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    owner_id        INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    path            TEXT    NOT NULL,
    file_name       TEXT    NOT NULL DEFAULT '',
    size_bytes      INTEGER NOT NULL DEFAULT 0,
    title           TEXT    NOT NULL DEFAULT '',
    original_title  TEXT    NOT NULL DEFAULT '',
    year            INTEGER NOT NULL DEFAULT 0,
    kind            TEXT    NOT NULL DEFAULT 'movie'
                    CHECK (kind IN ('movie', 'series', 'anime', 'documentary')),
    season          INTEGER NOT NULL DEFAULT 0,
    episode         INTEGER NOT NULL DEFAULT 0,
    overview        TEXT    NOT NULL DEFAULT '',
    poster          TEXT    NOT NULL DEFAULT '',
    backdrop        TEXT    NOT NULL DEFAULT '',
    rating          REAL    NOT NULL DEFAULT 0,
    runtime_seconds INTEGER NOT NULL DEFAULT 0,
    genres          TEXT    NOT NULL DEFAULT '',
    cast_list       TEXT    NOT NULL DEFAULT '',
    tmdb_id         INTEGER NOT NULL DEFAULT 0,
    resolution      TEXT    NOT NULL DEFAULT '',
    video_codec     TEXT    NOT NULL DEFAULT '',
    audio_codec     TEXT    NOT NULL DEFAULT '',
    scraped         INTEGER NOT NULL DEFAULT 0,
    created_at      TEXT    NOT NULL,
    UNIQUE (owner_id, path)
);
CREATE INDEX IF NOT EXISTS idx_movies_owner ON movies(owner_id);

CREATE TABLE IF NOT EXISTS apps (
    slug            TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    category        TEXT NOT NULL DEFAULT 'tools',
    description     TEXT NOT NULL DEFAULT '',
    icon            TEXT NOT NULL DEFAULT '',
    image           TEXT NOT NULL DEFAULT '',
    ports           TEXT NOT NULL DEFAULT '[]',
    compose         TEXT NOT NULL DEFAULT '',
    installed       INTEGER NOT NULL DEFAULT 0,
    status          TEXT NOT NULL DEFAULT 'stopped',
    installed_at    TEXT,
    updated_at      TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS network_shares (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    path        TEXT NOT NULL,
    comment     TEXT NOT NULL DEFAULT '',
    protocols   TEXT NOT NULL DEFAULT '["smb"]',
    read_only   INTEGER NOT NULL DEFAULT 0,
    valid_users TEXT NOT NULL DEFAULT '@users',
    guest_ok    INTEGER NOT NULL DEFAULT 0,
    created_at  TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS destroy_events (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at  TEXT NOT NULL,
    username    TEXT NOT NULL DEFAULT '',
    trigger     TEXT NOT NULL DEFAULT '',
    method      TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT '',
    detail      TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_destroy_created ON destroy_events(created_at);

CREATE TABLE IF NOT EXISTS notifications (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    level       TEXT NOT NULL DEFAULT 'info',
    title       TEXT NOT NULL,
    message     TEXT NOT NULL DEFAULT '',
    read        INTEGER NOT NULL DEFAULT 0,
    created_at  TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_notifications_read ON notifications(read, created_at);

CREATE TABLE IF NOT EXISTS alert_events (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    key         TEXT NOT NULL,
    level       TEXT NOT NULL DEFAULT 'warning',
    title       TEXT NOT NULL,
    message     TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL,
    resolved_at TEXT
);
CREATE INDEX IF NOT EXISTS idx_alert_key ON alert_events(key, created_at);

PRAGMA user_version = 1;
