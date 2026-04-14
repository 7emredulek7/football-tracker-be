package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	CalendarRequestStatusPending     = "pending"
	CalendarRequestStatusAccepted    = "accepted"
	CalendarRequestStatusRejected    = "rejected"
	CalendarRequestStatusRescheduled = "rescheduled"
)

type CalendarRequest struct {
	ID            primitive.ObjectID  `bson:"_id,omitempty" json:"id"`
	Opponent      string              `bson:"opponent" json:"opponent"`
	RequestedDate time.Time           `bson:"requestedDate" json:"requestedDate"`
	ScheduledDate *time.Time          `bson:"scheduledDate,omitempty" json:"scheduledDate,omitempty"`
	Status        string              `bson:"status" json:"status"`
	MatchID       *primitive.ObjectID `bson:"matchId,omitempty" json:"matchId,omitempty"`
	CreatedAt     time.Time           `bson:"createdAt" json:"createdAt"`
	UpdatedAt     time.Time           `bson:"updatedAt" json:"updatedAt"`
}
