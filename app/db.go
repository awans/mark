package app

import (
	"crypto/rsa"

	"github.com/awans/mark/entities"
	"github.com/awans/mark/feed"
)

// DB is the application-level DB interface
type DB struct {
	key *rsa.PrivateKey // still maybe hide this in a Session
	e   *entities.DB
}

// NewDB makes a new app db from an entity db
func NewDB(e *entities.DB, key *rsa.PrivateKey) *DB {
	return &DB{e: e, key: key}
}

// Close closes the underlying db
func (db *DB) Close() {
	db.e.Close()
}

// GetFeed returns a user's feed
func (db *DB) GetFeed() ([]Bookmark, error) {
	var bookmarks []Bookmark
	db.e.GetAll(&bookmarks)
	return bookmarks, nil
}

// AddBookmark inserts a bookmark into the db
func (db *DB) AddBookmark(b Bookmark) {
	db.e.Add(b)
}

// UserFeed returns the user's feed
func (db *DB) UserFeed() (*feed.Feed, error) {
	return db.e.UserFeed()
}