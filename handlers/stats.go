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

type PlayerStats struct {
	Goals         int     `json:"goals"`
	Assists       int     `json:"assists"`
	MatchesPlayed int     `json:"matchesPlayed"`
	AverageRating float64 `json:"averageRating"`
}

func GetPlayerStats(c *gin.Context) {
	idParam := c.Param("id")
	playerID, err := primitive.ObjectIDFromHex(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid player ID"})
		return
	}

	collection := config.DB.Collection("matches")
	var matches []models.Match

	// Get all matches
	cursor, err := collection.Find(context.Background(), bson.M{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch matches"})
		return
	}
	defer cursor.Close(context.Background())
	if err = cursor.All(context.Background(), &matches); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch matches"})
		return
	}

	var stats PlayerStats
	var totalRating int
	var ratingCount int

	for _, match := range matches {
		played := false
		for _, lineupItem := range match.Lineup {
			if lineupItem.PlayerID == playerID {
				played = true
				stats.MatchesPlayed++
				break
			}
		}

		// Calculate goals & assists
		for _, event := range match.Events {
			if event.Type == "goal" && event.PlayerID == playerID {
				stats.Goals++
			}
			if event.Type == "assist" && event.PlayerID == playerID {
				stats.Assists++
			}
		}

		// Calculate average match rating across all owners
		if played {
			matchScoreTotal := 0
			matchScoreCount := 0
			for _, rating := range match.Ratings {
				for _, pScore := range rating.Scores {
					if pScore.PlayerID == playerID {
						matchScoreTotal += pScore.Score
						matchScoreCount++
					}
				}
			}

			if matchScoreCount > 0 {
				totalRating += matchScoreTotal / matchScoreCount
				ratingCount++
			}
		}
	}

	if ratingCount > 0 {
		stats.AverageRating = float64(totalRating) / float64(ratingCount)
	}

	c.JSON(http.StatusOK, stats)
}
