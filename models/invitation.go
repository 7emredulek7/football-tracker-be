package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Invitation struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	PlayerID  primitive.ObjectID `bson:"playerId"      json:"playerId"`
	Token     string             `bson:"token"         json:"token"`
	Used      bool               `bson:"used"          json:"used"`
	ExpiresAt time.Time          `bson:"expiresAt"     json:"expiresAt"`
	CreatedAt time.Time          `bson:"createdAt"     json:"createdAt"`
}
