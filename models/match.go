package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type MatchEvent struct {
	Type           string             `bson:"type" json:"type"` // "goal", "assist"
	PlayerID       primitive.ObjectID `bson:"playerId" json:"playerId"`
	AssistPlayerID primitive.ObjectID `bson:"assistPlayerId,omitempty" json:"assistPlayerId,omitempty"` // For goals
}

type MatchRating struct {
	OwnerID primitive.ObjectID `bson:"ownerId" json:"userId"`
	Scores  []PlayerScore      `bson:"scores" json:"scores"`
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
	ID       primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Date     time.Time          `bson:"date" json:"date"`
	Opponent string             `bson:"opponent" json:"opponent"`
	Lineup   []LineupEntry      `bson:"lineup" json:"lineup"`
	Events   []MatchEvent       `bson:"events" json:"events"`
	Ratings  []MatchRating      `bson:"ratings" json:"ratings"`
	Score    MatchScore         `bson:"score" json:"score"`
	Result   string             `bson:"result" json:"result"` // "Win", "Loss", "Draw"
}
