package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type MatchEvent struct {
	Type     string             `bson:"type" json:"type"` // "goal", "assist"
	PlayerID primitive.ObjectID `bson:"playerId" json:"playerId"`
}

type WatcherEntry struct {
	Name  string `bson:"name"  json:"name"`
	Token string `bson:"token" json:"token"`
	Used  bool   `bson:"used"  json:"used"`
}

type GuestRatingEntry struct {
	PlayerID   primitive.ObjectID `bson:"playerId"   json:"playerId"`
	PlayerName string             `bson:"playerName" json:"playerName"`
	Token      string             `bson:"token"      json:"token"`
	Used       bool               `bson:"used"       json:"used"`
}

type MatchRating struct {
	OwnerID     *primitive.ObjectID `bson:"ownerId,omitempty"     json:"userId,omitempty"`
	WatcherName string              `bson:"watcherName,omitempty" json:"watcherName,omitempty"`
	GuestName   string              `bson:"guestName,omitempty"   json:"guestName,omitempty"`
	RaterType   string              `bson:"raterType"             json:"raterType"` // "player" | "watcher" | "guest"
	Scores      []PlayerScore       `bson:"scores"                json:"scores"`
}

type PlayerScore struct {
	PlayerID primitive.ObjectID `bson:"playerId" json:"playerId"`
	Score    int                `bson:"score" json:"score"` // 0-10
}

type MatchScore struct {
	For     int `bson:"for" json:"for"`
	Against int `bson:"against" json:"against"`
}

type Match struct {
	ID                primitive.ObjectID  `bson:"_id,omitempty" json:"id"`
	Date              time.Time           `bson:"date" json:"date"`
	Opponent          string              `bson:"opponent" json:"opponent"`
	Lineup            []LineupEntry       `bson:"lineup" json:"lineup"`
	Events            []MatchEvent        `bson:"events" json:"events"`
	Ratings           []MatchRating       `bson:"ratings"           json:"ratings"`
	Watchers          []WatcherEntry      `bson:"watchers"          json:"watchers"`
	GuestRatingTokens []GuestRatingEntry  `bson:"guestRatingTokens" json:"guestRatingTokens"`
	Score             MatchScore          `bson:"score"             json:"score"`
	Result            string              `bson:"result"            json:"result"` // "Win", "Loss", "Draw"
}
