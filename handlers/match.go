package handlers

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/user/7aside-tracker/config"
	"github.com/user/7aside-tracker/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func GetMatches(c *gin.Context) {
	collection := config.DB.Collection("matches")
	var matches []models.Match

	cursor, err := collection.Find(context.Background(), bson.M{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch matches"})
		return
	}
	defer cursor.Close(context.Background())

	if err = cursor.All(context.Background(), &matches); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decode matches"})
		return
	}

	if matches == nil {
		matches = []models.Match{}
	}

	c.JSON(http.StatusOK, matches)
}

func GetMatchByID(c *gin.Context) {
	idParam := c.Param("id")
	objectID, err := primitive.ObjectIDFromHex(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid match ID"})
		return
	}

	collection := config.DB.Collection("matches")
	var match models.Match

	err = collection.FindOne(context.Background(), bson.M{"_id": objectID}).Decode(&match)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Match not found"})
		return
	}

	c.JSON(http.StatusOK, match)
}

func CreateMatch(c *gin.Context) {
	var match models.Match
	if err := c.ShouldBindJSON(&match); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	match.ID = primitive.NewObjectID()
	if match.Events == nil {
		match.Events = []models.MatchEvent{}
	}
	if match.Ratings == nil {
		match.Ratings = []models.MatchRating{}
	}

	collection := config.DB.Collection("matches")
	_, err := collection.InsertOne(context.Background(), match)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create match"})
		return
	}

	c.JSON(http.StatusCreated, match)
}

func UpdateMatch(c *gin.Context) {
	idParam := c.Param("id")
	objectID, err := primitive.ObjectIDFromHex(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid match ID"})
		return
	}

	var updateData map[string]interface{}
	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Remove ID so it can't be overwritten
	delete(updateData, "_id")
	delete(updateData, "id")

	collection := config.DB.Collection("matches")
	update := bson.M{
		"$set": updateData,
	}

	_, err = collection.UpdateOne(context.Background(), bson.M{"_id": objectID}, update)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update match"})
		return
	}

	var match models.Match
	err = collection.FindOne(context.Background(), bson.M{"_id": objectID}).Decode(&match)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "Match updated"})
		return
	}

	c.JSON(http.StatusOK, match)
}

func AddEvents(c *gin.Context) {
	idParam := c.Param("id")
	objectID, err := primitive.ObjectIDFromHex(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid match ID"})
		return
	}

	var newEvents []models.MatchEvent
	if err := c.ShouldBindJSON(&newEvents); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	collection := config.DB.Collection("matches")
	update := bson.M{
		"$push": bson.M{
			"events": bson.M{"$each": newEvents},
		},
	}

	_, err = collection.UpdateOne(context.Background(), bson.M{"_id": objectID}, update)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add events"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Events added successfully"})
}

func AddRatings(c *gin.Context) {
	idParam := c.Param("id")
	objectID, err := primitive.ObjectIDFromHex(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid match ID"})
		return
	}

	var rating models.MatchRating
	if err := c.ShouldBindJSON(&rating); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Make sure OwnerID is set
	ownerIdValue, exists := c.Get("userId")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	ownerID, _ := primitive.ObjectIDFromHex(ownerIdValue.(string))
	rating.OwnerID = ownerID

	collection := config.DB.Collection("matches")

	// First check if this owner has already voted
	// If MVP doesn't need to prevent double votes perfectly, we can just push.
	// But let's just push for simplicity, or we can use $pull to remove old vote then $push

	// remove old rating from this owner
	_, _ = collection.UpdateOne(
		context.Background(),
		bson.M{"_id": objectID},
		bson.M{"$pull": bson.M{"ratings": bson.M{"ownerId": ownerID}}},
	)

	// push new rating
	update := bson.M{
		"$push": bson.M{
			"ratings": rating,
		},
	}

	_, err = collection.UpdateOne(context.Background(), bson.M{"_id": objectID}, update)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add ratings"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Ratings added successfully"})
}

func DeleteMatch(c *gin.Context) {
	idParam := c.Param("id")
	objectID, err := primitive.ObjectIDFromHex(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid match ID"})
		return
	}

	collection := config.DB.Collection("matches")

	_, err = collection.DeleteOne(context.Background(), bson.M{"_id": objectID})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete match"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Match deleted successfully"})
}
