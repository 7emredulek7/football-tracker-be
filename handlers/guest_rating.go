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

// GenerateGuestRatingLinks generates one-time rating tokens for guest players in the lineup.
// POST /api/matches/:id/guest-ratings
func GenerateGuestRatingLinks(c *gin.Context) {
	idParam := c.Param("id")
	matchID, err := primitive.ObjectIDFromHex(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid match ID"})
		return
	}

	collection := config.DB.Collection("matches")
	playerCollection := config.DB.Collection("players")

	var match models.Match
	if err := collection.FindOne(context.Background(), bson.M{"_id": matchID}).Decode(&match); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Match not found"})
		return
	}

	var lineupPlayerIDs []primitive.ObjectID
	for _, entry := range match.Lineup {
		lineupPlayerIDs = append(lineupPlayerIDs, entry.PlayerID)
	}

	cursor, err := playerCollection.Find(context.Background(), bson.M{
		"_id":     bson.M{"$in": lineupPlayerIDs},
		"isGuest": true,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch players"})
		return
	}
	defer cursor.Close(context.Background())

	var guestPlayers []models.Player
	if err := cursor.All(context.Background(), &guestPlayers); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decode players"})
		return
	}

	if len(guestPlayers) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No guest players in the lineup"})
		return
	}

	existingTokens := make(map[string]bool)
	for _, entry := range match.GuestRatingTokens {
		existingTokens[entry.PlayerID.Hex()] = true
	}

	frontendURL := strings.TrimRight(os.Getenv("FRONTEND_URL"), "/")

	type GuestRatingResult struct {
		PlayerID   string `json:"playerId"`
		PlayerName string `json:"playerName"`
		Token      string `json:"token"`
		URL        string `json:"url"`
		Used       bool   `json:"used"`
	}
	var results []GuestRatingResult

	for _, player := range guestPlayers {
		if existingTokens[player.ID.Hex()] {
			for _, entry := range match.GuestRatingTokens {
				if entry.PlayerID == player.ID {
					results = append(results, GuestRatingResult{
						PlayerID:   player.ID.Hex(),
						PlayerName: player.FirstName + " " + player.LastName,
						Token:      entry.Token,
						URL:        frontendURL + "/guest-rate/" + entry.Token,
						Used:       entry.Used,
					})
					break
				}
			}
			continue
		}

		tokenBytes := make([]byte, 32)
		if _, err := rand.Read(tokenBytes); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
			return
		}
		tokenStr := hex.EncodeToString(tokenBytes)
		playerName := player.FirstName + " " + player.LastName

		entry := models.GuestRatingEntry{
			PlayerID:   player.ID,
			PlayerName: playerName,
			Token:      tokenStr,
			Used:       false,
		}

		_, err = collection.UpdateOne(
			context.Background(),
			bson.M{"_id": matchID},
			bson.M{"$push": bson.M{"guestRatingTokens": entry}},
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save guest rating token"})
			return
		}

		results = append(results, GuestRatingResult{
			PlayerID:   player.ID.Hex(),
			PlayerName: playerName,
			Token:      tokenStr,
			URL:        frontendURL + "/guest-rate/" + tokenStr,
			Used:       entry.Used,
		})
	}

	c.JSON(http.StatusCreated, results)
}

// GetGuestRatingContext returns match info for a guest player's rating page.
// GET /api/guest-ratings/:token
func GetGuestRatingContext(c *gin.Context) {
	tokenStr := c.Param("token")

	collection := config.DB.Collection("matches")
	var match models.Match

	err := collection.FindOne(
		context.Background(),
		bson.M{"guestRatingTokens.token": tokenStr},
	).Decode(&match)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Invalid or expired link"})
		return
	}

	var guestEntry *models.GuestRatingEntry
	for i := range match.GuestRatingTokens {
		if match.GuestRatingTokens[i].Token == tokenStr {
			guestEntry = &match.GuestRatingTokens[i]
			break
		}
	}
	if guestEntry == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Guest rating entry not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"matchId":      match.ID.Hex(),
		"opponent":     match.Opponent,
		"date":         match.Date,
		"lineup":       match.Lineup,
		"playerName":   guestEntry.PlayerName,
		"playerId":     guestEntry.PlayerID.Hex(),
		"alreadyRated": guestEntry.Used,
	})
}

// SubmitGuestRating allows a guest player to submit ratings via their unique token.
// POST /api/guest-ratings/:token/ratings
func SubmitGuestRating(c *gin.Context) {
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
		bson.M{"guestRatingTokens.token": tokenStr},
	).Decode(&match)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Invalid or expired link"})
		return
	}

	var guestEntry *models.GuestRatingEntry
	for i := range match.GuestRatingTokens {
		if match.GuestRatingTokens[i].Token == tokenStr {
			guestEntry = &match.GuestRatingTokens[i]
			break
		}
	}
	if guestEntry == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Guest rating entry not found"})
		return
	}

	if guestEntry.Used {
		c.JSON(http.StatusConflict, gin.H{"error": "You have already submitted ratings for this match"})
		return
	}

	lineupSet := make(map[string]bool)
	for _, entry := range match.Lineup {
		lineupSet[entry.PlayerID.Hex()] = true
	}

	for _, score := range req.Scores {
		if !lineupSet[score.PlayerID.Hex()] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Rating contains a player not in the lineup"})
			return
		}
		if score.PlayerID == guestEntry.PlayerID {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Players cannot rate themselves"})
			return
		}
	}

	rating := models.MatchRating{
		GuestName: guestEntry.PlayerName,
		RaterType: "guest",
		Scores:    req.Scores,
	}

	// Remove any prior rating from this guest, then push new
	_, _ = collection.UpdateOne(
		context.Background(),
		bson.M{"_id": match.ID},
		bson.M{"$pull": bson.M{"ratings": bson.M{"guestName": guestEntry.PlayerName, "raterType": "guest"}}},
	)

	_, err = collection.UpdateOne(
		context.Background(),
		bson.M{"_id": match.ID, "guestRatingTokens.token": tokenStr},
		bson.M{
			"$push": bson.M{"ratings": rating},
			"$set":  bson.M{"guestRatingTokens.$.used": true},
		},
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save ratings"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Ratings submitted successfully"})
}
