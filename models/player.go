package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Player struct {
	ID        primitive.ObjectID  `bson:"_id,omitempty" json:"id"`
	FirstName string              `bson:"firstName" json:"firstName"`
	LastName  string              `bson:"lastName" json:"lastName"`
	Number    int                 `bson:"number" json:"number"`
	IsGuest   bool                `bson:"isGuest" json:"isGuest"`
	UserID    *primitive.ObjectID `bson:"userId,omitempty" json:"userId,omitempty"`
	CreatedAt time.Time           `bson:"createdAt" json:"createdAt"`
}
