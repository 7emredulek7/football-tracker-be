package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/user/7aside-tracker/config"
	"github.com/user/7aside-tracker/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AddWatchers — owner adds named watchers to a match; generates a unique token per watcher.
// POST /api/matches/:id/watchers
func AddWatchers(c *gin.Context) {
	idParam := c.Param("id")
	matchID, err := primitive.ObjectIDFromHex(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid match ID"})
		return
	}

	var req struct {
		Names []string `json:"names" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Names) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "names array is required"})
		return
	}

	frontendURL := strings.TrimRight(os.Getenv("FRONTEND_URL"), "/")

	collection := config.DB.Collection("matches")

	type WatcherResult struct {
		Name  string `json:"name"`
		Token string `json:"token"`
		URL   string `json:"url"`
	}
	var results []WatcherResult

	for _, name := range req.Names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		tokenBytes := make([]byte, 32)
		if _, err := rand.Read(tokenBytes); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
			return
		}
		tokenStr := hex.EncodeToString(tokenBytes)

		entry := models.WatcherEntry{
			Name:  name,
			Token: tokenStr,
			Used:  false,
		}

		_, err = collection.UpdateOne(
			context.Background(),
			bson.M{"_id": matchID},
			bson.M{"$push": bson.M{"watchers": entry}},
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add watcher"})
			return
		}

		results = append(results, WatcherResult{
			Name:  name,
			Token: tokenStr,
			URL:   frontendURL + "/rate/" + tokenStr,
		})
	}

	c.JSON(http.StatusCreated, results)
}

// GetWatcherContext — public endpoint; returns match info for a watcher's rating page.
// GET /api/watchers/:token
func GetWatcherContext(c *gin.Context) {
	tokenStr := c.Param("token")

	collection := config.DB.Collection("matches")
	var match models.Match

	err := collection.FindOne(
		context.Background(),
		bson.M{"watchers.token": tokenStr},
	).Decode(&match)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Invalid or expired link"})
		return
	}

	// Find the specific watcher entry
	var watcher *models.WatcherEntry
	for i := range match.Watchers {
		if match.Watchers[i].Token == tokenStr {
			watcher = &match.Watchers[i]
			break
		}
	}
	if watcher == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Watcher not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"matchId":      match.ID.Hex(),
		"opponent":     match.Opponent,
		"date":         match.Date,
		"lineup":       match.Lineup,
		"watcherName":  watcher.Name,
		"alreadyRated": watcher.Used,
	})
}

// SubmitWatcherRatings — public endpoint; watcher submits ratings via their unique token.
// POST /api/watchers/:token/ratings
func SubmitWatcherRatings(c *gin.Context) {
	tokenStr := c.Param("token")

	var req struct {
		Scores []models.PlayerScore `json:"scores" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	collection := config.DB.Collection("matches")
	var match models.Match

	err := collection.FindOne(
		context.Background(),
		bson.M{"watchers.token": tokenStr},
	).Decode(&match)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Invalid or expired link"})
		return
	}

	// Find watcher entry
	var watcher *models.WatcherEntry
	for i := range match.Watchers {
		if match.Watchers[i].Token == tokenStr {
			watcher = &match.Watchers[i]
			break
		}
	}
	if watcher == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Watcher not found"})
		return
	}

	if watcher.Used {
		c.JSON(http.StatusConflict, gin.H{"error": "You have already submitted ratings for this match"})
		return
	}

	// Build lineup set for validation
	lineupSet := make(map[string]bool)
	for _, entry := range match.Lineup {
		lineupSet[entry.PlayerID.Hex()] = true
	}

	for _, score := range req.Scores {
		if !lineupSet[score.PlayerID.Hex()] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Rating contains a player not in the lineup"})
			return
		}
	}

	rating := models.MatchRating{
		WatcherName: watcher.Name,
		RaterType:   "watcher",
		Scores:      req.Scores,
	}

	// Remove any prior rating from this watcher (idempotency), then push new
	_, _ = collection.UpdateOne(
		context.Background(),
		bson.M{"_id": match.ID},
		bson.M{"$pull": bson.M{"ratings": bson.M{"watcherName": watcher.Name, "raterType": "watcher"}}},
	)

	_, err = collection.UpdateOne(
		context.Background(),
		bson.M{"_id": match.ID, "watchers.token": tokenStr},
		bson.M{
			"$push": bson.M{"ratings": rating},
			"$set":  bson.M{"watchers.$.used": true},
		},
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save ratings"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Ratings submitted successfully"})
}
