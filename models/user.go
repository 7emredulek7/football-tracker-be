package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type User struct {
	ID           primitive.ObjectID  `bson:"_id,omitempty" json:"id"`
	Email        string              `bson:"email" json:"email"`
	PasswordHash string              `bson:"passwordHash" json:"-"`
	Role         string              `bson:"role" json:"role"` // "owner" or "player"
	PlayerID     *primitive.ObjectID `bson:"playerId,omitempty" json:"playerId,omitempty"`
	CreatedAt    time.Time           `bson:"createdAt" json:"createdAt"`
}
