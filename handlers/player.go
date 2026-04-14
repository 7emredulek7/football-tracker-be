package handlers

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/user/7aside-tracker/config"
	"github.com/user/7aside-tracker/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type PlayerWithStats struct {
	models.Player
	Stats PlayerStats `json:"stats"`
}

func GetPlayers(c *gin.Context) {
	collection := config.DB.Collection("players")
	var players []models.Player

	cursor, err := collection.Find(context.Background(), bson.M{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch players"})
		return
	}
	defer cursor.Close(context.Background())

	if err = cursor.All(context.Background(), &players); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decode players"})
		return
	}

	if players == nil {
		players = []models.Player{}
	}

	c.JSON(http.StatusOK, players)
}

func GetPlayersWithStats(c *gin.Context) {
	playersCol := config.DB.Collection("players")
	matchesCol := config.DB.Collection("matches")

	var players []models.Player
	cursor, err := playersCol.Find(context.Background(), bson.M{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch players"})
		return
	}
	defer cursor.Close(context.Background())
	if err = cursor.All(context.Background(), &players); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decode players"})
		return
	}

	var matches []models.Match
	mCursor, err := matchesCol.Find(context.Background(), bson.M{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch matches"})
		return
	}
	defer mCursor.Close(context.Background())
	if err = mCursor.All(context.Background(), &matches); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decode matches"})
		return
	}

	type statAcc struct {
		Goals       int
		Assists     int
		MatchPlayed int
		TotalRating float64
		RatingCount int
	}

	statsMap := map[primitive.ObjectID]*statAcc{}
	for _, p := range players {
		statsMap[p.ID] = &statAcc{}
	}

	for _, m := range matches {
		playedSet := map[primitive.ObjectID]bool{}
		for _, li := range m.Lineup {
			if _, ok := statsMap[li.PlayerID]; ok {
				if !playedSet[li.PlayerID] {
					statsMap[li.PlayerID].MatchPlayed++
					playedSet[li.PlayerID] = true
				}
			}
		}

		for _, ev := range m.Events {
			if acc, ok := statsMap[ev.PlayerID]; ok {
				switch ev.Type {
				case "goal":
					acc.Goals++
				case "assist":
					acc.Assists++
				}
			}
		}

		// Ratings: accumulate per-match average for each player
		matchRatings := map[primitive.ObjectID]struct{ total, count int }{}
		for _, r := range m.Ratings {
			for _, s := range r.Scores {
				if _, ok := statsMap[s.PlayerID]; ok {
					entry := matchRatings[s.PlayerID]
					entry.total += s.Score
					entry.count++
					matchRatings[s.PlayerID] = entry
				}
			}
		}
		for pid, mr := range matchRatings {
			if mr.count > 0 {
				statsMap[pid].TotalRating += float64(mr.total) / float64(mr.count)
				statsMap[pid].RatingCount++
			}
		}
	}

	result := make([]PlayerWithStats, len(players))
	for i, p := range players {
		acc := statsMap[p.ID]
		result[i].Player = p
		result[i].Stats.Goals = acc.Goals
		result[i].Stats.Assists = acc.Assists
		result[i].Stats.MatchesPlayed = acc.MatchPlayed
		if acc.RatingCount > 0 {
			result[i].Stats.AverageRating = acc.TotalRating / float64(acc.RatingCount)
		}
	}

	sortPlayersWithStats(result)

	c.JSON(http.StatusOK, result)
}

func sortPlayersWithStats(players []PlayerWithStats) {
	sort.SliceStable(players, func(i, j int) bool {
		left := players[i]
		right := players[j]

		if left.IsGuest != right.IsGuest {
			return !left.IsGuest
		}
		if left.Stats.AverageRating != right.Stats.AverageRating {
			return left.Stats.AverageRating > right.Stats.AverageRating
		}
		if left.Stats.MatchesPlayed != right.Stats.MatchesPlayed {
			return left.Stats.MatchesPlayed > right.Stats.MatchesPlayed
		}
		if left.Number != right.Number {
			return left.Number < right.Number
		}

		leftName := strings.ToLower(strings.TrimSpace(left.FirstName + " " + left.LastName))
		rightName := strings.ToLower(strings.TrimSpace(right.FirstName + " " + right.LastName))
		if leftName != rightName {
			return leftName < rightName
		}

		return left.ID.Hex() < right.ID.Hex()
	})
}

func CreatePlayer(c *gin.Context) {
	var player models.Player
	if err := c.ShouldBindJSON(&player); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	player.ID = primitive.NewObjectID()
	player.CreatedAt = time.Now()

	collection := config.DB.Collection("players")
	_, err := collection.InsertOne(context.Background(), player)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create player"})
		return
	}

	c.JSON(http.StatusCreated, player)
}

func UpdatePlayer(c *gin.Context) {
	idParam := c.Param("id")
	objectID, err := primitive.ObjectIDFromHex(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid player ID"})
		return
	}

	var updateData struct {
		FirstName string `json:"firstName"`
		LastName  string `json:"lastName"`
		Number    int    `json:"number"`
		IsGuest   bool   `json:"isGuest"`
	}

	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	collection := config.DB.Collection("players")
	update := bson.M{
		"$set": bson.M{
			"firstName": updateData.FirstName,
			"lastName":  updateData.LastName,
			"number":    updateData.Number,
			"isGuest":   updateData.IsGuest,
		},
	}

	_, err = collection.UpdateOne(context.Background(), bson.M{"_id": objectID}, update)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update player"})
		return
	}

	var player models.Player
	err = collection.FindOne(context.Background(), bson.M{"_id": objectID}).Decode(&player)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "Player updated"})
		return
	}

	c.JSON(http.StatusOK, player)
}

func DeletePlayer(c *gin.Context) {
	idParam := c.Param("id")
	objectID, err := primitive.ObjectIDFromHex(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid player ID"})
		return
	}

	matchesCollection := config.DB.Collection("matches")

	// Verify if the player was part of any match lineup
	count, err := matchesCollection.CountDocuments(context.Background(), bson.M{
		"lineup.playerId": objectID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check matches"})
		return
	}

	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "En az bir maçta oynayan oyuncular silinemez."})
		return
	}

	collection := config.DB.Collection("players")

	// Find the player to check for a linked user account
	var player models.Player
	if err := collection.FindOne(context.Background(), bson.M{"_id": objectID}).Decode(&player); err == nil {
		usersCollection := config.DB.Collection("users")

		// Find linked non-owner user (match by playerId field on user)
		var linkedUser models.User
		err := usersCollection.FindOne(context.Background(), bson.M{"playerId": objectID}).Decode(&linkedUser)
		if err == nil && linkedUser.Role != "owner" {
			usersCollection.DeleteOne(context.Background(), bson.M{"_id": linkedUser.ID})
		}
	}

	res, err := collection.DeleteOne(context.Background(), bson.M{"_id": objectID})
	if err != nil || res.DeletedCount == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete player"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Player deleted successfully"})
}
