package handlers

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/user/7aside-tracker/config"
	"github.com/user/7aside-tracker/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func GetSettings(c *gin.Context) {
	collection := config.DB.Collection("settings")
	var settings models.Settings

	err := collection.FindOne(context.Background(), bson.M{"_id": "team-settings"}).Decode(&settings)
	if err != nil {
		// Use empty if missing
		settings = models.Settings{
			ID:               "team-settings",
			DefaultFormation: "3-2-1",
			DefaultLineup:    []models.LineupEntry{},
		}
	}

	c.JSON(http.StatusOK, settings)
}

func UpdateSettings(c *gin.Context) {
	var settings models.Settings
	if err := c.ShouldBindJSON(&settings); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	settings.ID = "team-settings"

	collection := config.DB.Collection("settings")
	opts := options.Update().SetUpsert(true)
	update := bson.M{
		"$set": bson.M{
			"defaultFormation": settings.DefaultFormation,
			"defaultLineup":    settings.DefaultLineup,
		},
	}

	_, err := collection.UpdateOne(context.Background(), bson.M{"_id": "team-settings"}, update, opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update settings"})
		return
	}

	c.JSON(http.StatusOK, settings)
}
