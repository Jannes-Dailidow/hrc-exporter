// Package domain holds the types shared between the portal client, the
// persistence layer and the exporter. It has no dependencies.
package domain

import (
	"errors"
	"time"
)

// Credentials are the first-factor login data of the portal.
type Credentials struct {
	User     string // Personalnummer
	Password string
}

// Document is a single entry of the portal's document list.
type Document struct {
	ID   string // stable portal GUID
	Year int    // year the document is listed under
	Date time.Time
	Type string // Dokumentenart, e.g. "Entgeltabrechnung"
}

// Record describes a document that has been downloaded.
type Record struct {
	ID           string
	Year         int
	Date         time.Time
	Type         string
	File         string // slash separated, relative to the output directory
	SHA256       string
	Size         int64
	DownloadedAt time.Time
}

var (
	ErrInvalidCredentials = errors.New("invalid personnel number or password")
	ErrInvalidCode        = errors.New("invalid login code")
	ErrNotLoggedIn        = errors.New("not logged in")
	ErrSessionExpired     = errors.New("portal session expired")
)
