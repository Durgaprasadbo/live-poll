package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Option struct {
	ID   string `bson:"id" json:"id"`
	Text string `bson:"text" json:"text"`
}

type Poll struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Question     string             `bson:"question" json:"question"`
	Options      []Option           `bson:"options" json:"options"`
	OwnerID      primitive.ObjectID `bson:"owner_id" json:"owner_id"`
	IsActive     bool               `bson:"is_active" json:"is_active"`
	AllowMulti   bool               `bson:"allow_multi_vote" json:"allow_multi_vote"` // allow same browser to vote again (off by default)
	CreatedAt    time.Time          `bson:"created_at" json:"created_at"`
	ExpiresAt    *time.Time         `bson:"expires_at,omitempty" json:"expires_at,omitempty"`
}

// PollResult is what we send over the wire for live results.
type PollResult struct {
	PollID string         `json:"poll_id"`
	Counts map[string]int64 `json:"counts"` // option_id -> vote count
	Total  int64          `json:"total"`
}
