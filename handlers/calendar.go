package handlers

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/user/7aside-tracker/config"
	"github.com/user/7aside-tracker/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type calendarMatchPayload struct {
	Opponent string    `json:"opponent" binding:"required"`
	Date     time.Time `json:"date" binding:"required"`
}

type calendarDatePayload struct {
	Date time.Time `json:"date" binding:"required"`
}

type optionalCalendarDatePayload struct {
	Date *time.Time `json:"date"`
}

func GetCalendarRequests(c *gin.Context) {
	getCalendarRequests(c, bson.M{})
}

func GetPublicCalendarRequests(c *gin.Context) {
	getCalendarRequests(c, bson.M{
		"status": bson.M{"$in": []string{
			models.CalendarRequestStatusPending,
			models.CalendarRequestStatusRescheduled,
		}},
	})
}

func getCalendarRequests(c *gin.Context, filter bson.M) {
	collection := config.DB.Collection("calendar_requests")
	findOptions := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}})

	cursor, err := collection.Find(context.Background(), filter, findOptions)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch calendar requests"})
		return
	}
	defer cursor.Close(context.Background())

	var requests []models.CalendarRequest
	if err = cursor.All(context.Background(), &requests); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decode calendar requests"})
		return
	}

	if requests == nil {
		requests = []models.CalendarRequest{}
	}

	c.JSON(http.StatusOK, requests)
}

func CreateCalendarRequest(c *gin.Context) {
	payload, ok := bindCalendarPayload(c)
	if !ok {
		return
	}
	if !validateNotPastCalendarDay(c, payload.Date) {
		return
	}

	now := time.Now().UTC()
	request := models.CalendarRequest{
		ID:            primitive.NewObjectID(),
		Opponent:      payload.Opponent,
		RequestedDate: payload.Date,
		Status:        models.CalendarRequestStatusPending,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	collection := config.DB.Collection("calendar_requests")
	if _, err := collection.InsertOne(context.Background(), request); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create calendar request"})
		return
	}

	c.JSON(http.StatusCreated, request)
}

func CreateCalendarMatch(c *gin.Context) {
	payload, ok := bindCalendarPayload(c)
	if !ok {
		return
	}

	match := newCalendarMatch(payload.Opponent, payload.Date)
	collection := config.DB.Collection("matches")
	if _, err := collection.InsertOne(context.Background(), match); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create calendar match"})
		return
	}

	c.JSON(http.StatusCreated, match)
}

func AcceptCalendarRequest(c *gin.Context) {
	request, ok := findCalendarRequest(c)
	if !ok {
		return
	}

	if request.Status == models.CalendarRequestStatusAccepted {
		c.JSON(http.StatusConflict, gin.H{"error": "Calendar request is already accepted"})
		return
	}
	if request.Status == models.CalendarRequestStatusRejected {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Rejected calendar requests cannot be accepted"})
		return
	}

	payload, ok := bindOptionalCalendarDatePayload(c)
	if !ok {
		return
	}

	matchDate := request.RequestedDate
	if request.ScheduledDate != nil {
		matchDate = *request.ScheduledDate
	}
	if payload.Date != nil {
		matchDate = *payload.Date
	}

	match := newCalendarMatch(request.Opponent, matchDate)
	matches := config.DB.Collection("matches")
	if _, err := matches.InsertOne(context.Background(), match); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create match from request"})
		return
	}

	requests := config.DB.Collection("calendar_requests")
	now := time.Now().UTC()
	update := bson.M{
		"$set": bson.M{
			"status":        models.CalendarRequestStatusAccepted,
			"scheduledDate": matchDate,
			"matchId":       match.ID,
			"updatedAt":     now,
		},
	}

	if _, err := requests.UpdateOne(context.Background(), bson.M{"_id": request.ID}, update); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to accept calendar request"})
		return
	}

	request.Status = models.CalendarRequestStatusAccepted
	request.ScheduledDate = &matchDate
	request.MatchID = &match.ID
	request.UpdatedAt = now

	c.JSON(http.StatusOK, request)
}

func RejectCalendarRequest(c *gin.Context) {
	request, ok := findCalendarRequest(c)
	if !ok {
		return
	}

	if request.Status == models.CalendarRequestStatusAccepted {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Accepted calendar requests cannot be rejected"})
		return
	}

	updated, ok := updateCalendarRequestStatus(c, request.ID, bson.M{
		"status":    models.CalendarRequestStatusRejected,
		"updatedAt": time.Now().UTC(),
	})
	if !ok {
		return
	}

	c.JSON(http.StatusOK, updated)
}

func RescheduleCalendarRequest(c *gin.Context) {
	request, ok := findCalendarRequest(c)
	if !ok {
		return
	}

	if request.Status == models.CalendarRequestStatusAccepted || request.Status == models.CalendarRequestStatusRejected {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Only pending or rescheduled calendar requests can be rescheduled"})
		return
	}

	payload, ok := bindCalendarDatePayload(c)
	if !ok {
		return
	}

	updated, ok := updateCalendarRequestStatus(c, request.ID, bson.M{
		"scheduledDate": payload.Date,
		"status":        models.CalendarRequestStatusRescheduled,
		"updatedAt":     time.Now().UTC(),
	})
	if !ok {
		return
	}

	c.JSON(http.StatusOK, updated)
}

func bindCalendarPayload(c *gin.Context) (calendarMatchPayload, bool) {
	var payload calendarMatchPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return payload, false
	}

	payload.Opponent = strings.TrimSpace(payload.Opponent)
	if payload.Opponent == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Opponent name is required"})
		return payload, false
	}

	if !validateHourlyDate(c, payload.Date) {
		return payload, false
	}

	return payload, true
}

func bindCalendarDatePayload(c *gin.Context) (calendarDatePayload, bool) {
	var payload calendarDatePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return payload, false
	}

	if !validateHourlyDate(c, payload.Date) {
		return payload, false
	}

	return payload, true
}

func bindOptionalCalendarDatePayload(c *gin.Context) (optionalCalendarDatePayload, bool) {
	var payload optionalCalendarDatePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		if errors.Is(err, io.EOF) {
			return payload, true
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return payload, false
	}

	if payload.Date != nil && !validateHourlyDate(c, *payload.Date) {
		return payload, false
	}

	return payload, true
}

func validateHourlyDate(c *gin.Context, date time.Time) bool {
	if date.IsZero() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Date is required"})
		return false
	}

	if date.Minute() != 0 || date.Second() != 0 || date.Nanosecond() != 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Match time must be on the hour"})
		return false
	}

	return true
}

func validateNotPastCalendarDay(c *gin.Context, date time.Time) bool {
	location, err := time.LoadLocation("Europe/Istanbul")
	if err != nil {
		location = time.Local
	}

	requestDate := date.In(location)
	now := time.Now().In(location)
	requestDay := time.Date(requestDate.Year(), requestDate.Month(), requestDate.Day(), 0, 0, 0, 0, location)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)

	if requestDay.Before(today) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Match requests cannot be created for past days"})
		return false
	}

	return true
}

func findCalendarRequest(c *gin.Context) (models.CalendarRequest, bool) {
	idParam := c.Param("id")
	objectID, err := primitive.ObjectIDFromHex(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid calendar request ID"})
		return models.CalendarRequest{}, false
	}

	var request models.CalendarRequest
	collection := config.DB.Collection("calendar_requests")
	if err := collection.FindOne(context.Background(), bson.M{"_id": objectID}).Decode(&request); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Calendar request not found"})
		return models.CalendarRequest{}, false
	}

	return request, true
}

func updateCalendarRequestStatus(c *gin.Context, id primitive.ObjectID, fields bson.M) (models.CalendarRequest, bool) {
	collection := config.DB.Collection("calendar_requests")
	after := options.After
	findOptions := options.FindOneAndUpdate().SetReturnDocument(after)

	var updated models.CalendarRequest
	err := collection.FindOneAndUpdate(
		context.Background(),
		bson.M{"_id": id},
		bson.M{"$set": fields},
		findOptions,
	).Decode(&updated)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update calendar request"})
		return models.CalendarRequest{}, false
	}

	return updated, true
}

func newCalendarMatch(opponent string, date time.Time) models.Match {
	return models.Match{
		ID:                primitive.NewObjectID(),
		Date:              date,
		Opponent:          opponent,
		Lineup:            []models.LineupEntry{},
		Events:            []models.MatchEvent{},
		Ratings:           []models.MatchRating{},
		Watchers:          []models.WatcherEntry{},
		GuestRatingTokens: []models.GuestRatingEntry{},
		Score:             models.MatchScore{For: 0, Against: 0},
		Result:            "Draw",
	}
}
