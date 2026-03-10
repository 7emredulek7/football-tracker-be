package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type LineupEntry struct {
	Position string             `bson:"position" json:"position"` // e.g., "GK", "D1", "M1", "F1"
	PlayerID primitive.ObjectID `bson:"playerId" json:"playerId"`
}

type Settings struct {
	ID               string        `bson:"_id" json:"id"`                            // "team-settings"
	DefaultFormation string        `bson:"defaultFormation" json:"defaultFormation"` // e.g., "3-2-1"
	DefaultLineup    []LineupEntry `bson:"defaultLineup" json:"defaultLineup"`
}
