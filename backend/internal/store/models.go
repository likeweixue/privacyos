package store

import (
	"encoding/json"
	"time"
)

// Permissions is the per-user feature permission set.
type Permissions struct {
	Files  bool `json:"files"`
	Photos bool `json:"photos"`
	Movies bool `json:"movies"`
	Share  bool `json:"share"`
	Docker bool `json:"docker"`
	Apps   bool `json:"apps"`
}

// DefaultPermissions returns the default feature set for a role.
func DefaultPermissions(role string) Permissions {
	p := Permissions{Files: true, Photos: true, Movies: true, Share: true}
	if role == "admin" {
		p.Docker = true
		p.Apps = true
	}
	return p
}

// Preferences are per-user UI preferences.
type Preferences struct {
	Lang  string `json:"lang"`
	Theme string `json:"theme"`
}

// User is a PrivacyOS account.
type User struct {
	ID           int64       `json:"id"`
	Username     string      `json:"username"`
	DisplayName  string      `json:"display_name"`
	PasswordHash string      `json:"-"`
	Role         string      `json:"role"`
	Status       string      `json:"status"`
	QuotaBytes   int64       `json:"quota_bytes"`
	Permissions  Permissions `json:"permissions"`
	Preferences  Preferences `json:"preferences"`
	TOTPSecret   string      `json:"-"`
	TOTPEnabled  bool        `json:"totp_enabled"`
	LockedUntil  string      `json:"locked_until,omitempty"`
	CreatedAt    string      `json:"created_at"`
	UpdatedAt    string      `json:"updated_at"`
	LastLoginAt  string      `json:"last_login_at,omitempty"`
	LastLoginIP  string      `json:"last_login_ip,omitempty"`
	DeleteAfter  string      `json:"delete_after,omitempty"`
	// QuotaHidden is set by the API when the edition forbids showing another
	// user's quota to an administrator.
	QuotaHidden bool `json:"quota_hidden,omitempty"`
}

// Session is a login session bound to an access/refresh token pair.
type Session struct {
	ID        string `json:"id"`
	UserID    int64  `json:"user_id"`
	IP        string `json:"ip"`
	UserAgent string `json:"user_agent"`
	CreatedAt string `json:"created_at"`
	LastSeen  string `json:"last_seen"`
	ExpiresAt string `json:"expires_at"`
	RevokedAt string `json:"revoked_at,omitempty"`
}

// LoginAttempt is one authentication attempt.
type LoginAttempt struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	IP        string `json:"ip"`
	Success   bool   `json:"success"`
	Reason    string `json:"reason"`
	CreatedAt string `json:"created_at"`
}

// AuditLog is one audit record.
type AuditLog struct {
	ID        int64  `json:"id"`
	CreatedAt string `json:"created_at"`
	UserID    int64  `json:"user_id,omitempty"`
	Username  string `json:"username"`
	Category  string `json:"category"`
	Action    string `json:"action"`
	Target    string `json:"target"`
	Detail    string `json:"detail"`
	IP        string `json:"ip"`
	Result    string `json:"result"`
}

// Share is a public share link for a file, directory or media item.
type Share struct {
	ID           string `json:"id"`
	Token        string `json:"token"`
	Name         string `json:"name"`
	Path         string `json:"-"`
	OwnerID      int64  `json:"owner_id"`
	Kind         string `json:"kind"`
	PasswordHash string `json:"-"`
	HasPassword  bool   `json:"has_password"`
	ExpiresAt    string `json:"expires_at,omitempty"`
	DownloadOnly bool   `json:"download_only"`
	MaxDownloads int64  `json:"max_downloads"`
	Downloads    int64  `json:"downloads"`
	SizeBytes    int64  `json:"size_bytes"`
	CreatedAt    string `json:"created_at"`
	LastAccessAt string `json:"last_access_at,omitempty"`
	URL          string `json:"url,omitempty"`
}

// TrashItem is a soft-deleted file entry.
type TrashItem struct {
	ID           string `json:"id"`
	OwnerID      int64  `json:"owner_id"`
	OriginalPath string `json:"-"`
	TrashPath    string `json:"-"`
	Name         string `json:"name"`
	IsDir        bool   `json:"is_dir"`
	SizeBytes    int64  `json:"size_bytes"`
	DeletedAt    string `json:"deleted_at"`
	ExpiresAt    string `json:"expires_at,omitempty"`
}

// StoragePool is a software RAID / filesystem pool managed by PrivacyOS.
type StoragePool struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Level      string   `json:"level"`
	FS         string   `json:"fs"`
	Devices    []string `json:"devices"`
	Mountpoint string   `json:"mountpoint"`
	Status     string   `json:"status"`
	SizeBytes  int64    `json:"size_bytes"`
	CreatedAt  string   `json:"created_at"`
	UpdatedAt  string   `json:"updated_at"`
}

// SmartTest is a stored SMART self-test result.
type SmartTest struct {
	ID        int64  `json:"id"`
	Device    string `json:"device"`
	Kind      string `json:"kind"`
	Status    string `json:"status"`
	Summary   string `json:"summary"`
	Raw       string `json:"-"`
	CreatedAt string `json:"created_at"`
}

// Photo is an indexed photo.
type Photo struct {
	ID          int64   `json:"id"`
	OwnerID     int64   `json:"owner_id"`
	Path        string  `json:"-"`
	FileName    string  `json:"file_name"`
	SizeBytes   int64   `json:"size_bytes"`
	Width       int     `json:"width"`
	Height      int     `json:"height"`
	TakenAt     string  `json:"taken_at,omitempty"`
	Camera      string  `json:"camera,omitempty"`
	Lens        string  `json:"lens,omitempty"`
	GPSLat      float64 `json:"gps_lat,omitempty"`
	GPSLng      float64 `json:"gps_lng,omitempty"`
	Orientation int     `json:"orientation"`
	ThumbPath   string  `json:"-"`
	SHA256      string  `json:"sha256,omitempty"`
	CreatedAt   string  `json:"created_at"`
	// URL fields are filled by the API layer.
	FileURL  string `json:"file_url,omitempty"`
	ThumbURL string `json:"thumb_url,omitempty"`
}

// Album is a user-defined photo album.
type Album struct {
	ID          int64  `json:"id"`
	OwnerID     int64  `json:"owner_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CoverPhoto  int64  `json:"cover_photo,omitempty"`
	CreatedAt   string `json:"created_at"`
	PhotoCount  int64  `json:"photo_count"`
}

// Movie is an indexed movie or episode.
type Movie struct {
	ID             int64   `json:"id"`
	OwnerID        int64   `json:"owner_id"`
	Path           string  `json:"-"`
	FileName       string  `json:"file_name"`
	SizeBytes      int64   `json:"size_bytes"`
	Title          string  `json:"title"`
	OriginalTitle  string  `json:"original_title,omitempty"`
	Year           int     `json:"year"`
	Kind           string  `json:"kind"`
	Season         int     `json:"season,omitempty"`
	Episode        int     `json:"episode,omitempty"`
	Overview       string  `json:"overview,omitempty"`
	Poster         string  `json:"-"`
	Backdrop       string  `json:"-"`
	Rating         float64 `json:"rating"`
	RuntimeSeconds int     `json:"runtime_seconds"`
	Genres         string  `json:"genres,omitempty"`
	Cast           string  `json:"cast,omitempty"`
	TMDBID         int64   `json:"tmdb_id,omitempty"`
	Resolution     string  `json:"resolution,omitempty"`
	VideoCodec     string  `json:"video_codec,omitempty"`
	AudioCodec     string  `json:"audio_codec,omitempty"`
	Scraped        bool    `json:"scraped"`
	CreatedAt      string  `json:"created_at"`
	StreamURL      string  `json:"stream_url,omitempty"`
	PosterURL      string  `json:"poster_url,omitempty"`
	Playable       bool    `json:"playable"`
}

// App is an application-center catalog entry.
type App struct {
	Slug        string   `json:"slug"`
	Name        string   `json:"name"`
	Category    string   `json:"category"`
	Description string   `json:"description"`
	Icon        string   `json:"icon"`
	Image       string   `json:"image"`
	Ports       []string `json:"ports"`
	Compose     string   `json:"compose,omitempty"`
	Installed   bool     `json:"installed"`
	Status      string   `json:"status"`
	InstalledAt string   `json:"installed_at,omitempty"`
}

// NetworkShare is an SMB/NFS export definition.
type NetworkShare struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Path       string   `json:"path"`
	Comment    string   `json:"comment"`
	Protocols  []string `json:"protocols"`
	ReadOnly   bool     `json:"read_only"`
	ValidUsers string   `json:"valid_users"`
	GuestOK    bool     `json:"guest_ok"`
	CreatedAt  string   `json:"created_at"`
}

// DestroyEvent records a destructive policy action.
type DestroyEvent struct {
	ID        int64  `json:"id"`
	CreatedAt string `json:"created_at"`
	Username  string `json:"username"`
	Trigger   string `json:"trigger"`
	Method    string `json:"method"`
	Status    string `json:"status"`
	Detail    string `json:"detail"`
}

// Notification is an in-app notification.
type Notification struct {
	ID        int64  `json:"id"`
	Level     string `json:"level"`
	Title     string `json:"title"`
	Message   string `json:"message"`
	Read      bool   `json:"read"`
	CreatedAt string `json:"created_at"`
}

// AlertEvent is a raised system alert.
type AlertEvent struct {
	ID         int64  `json:"id"`
	Key        string `json:"key"`
	Level      string `json:"level"`
	Title      string `json:"title"`
	Message    string `json:"message"`
	CreatedAt  string `json:"created_at"`
	ResolvedAt string `json:"resolved_at,omitempty"`
}

// Now returns the canonical timestamp format used across the store.
func Now() string { return time.Now().UTC().Format(time.RFC3339) }

// Time parses a stored timestamp; the zero value is returned on error.
func Time(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

// JSONMap marshals v to a JSON string, returning "{}" on error.
func JSONMap(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}
