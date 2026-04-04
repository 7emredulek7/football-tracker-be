package handlers

import (
	"context"
	"net/http"
	"time"

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
	var totalRating float64
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
				totalRating += float64(matchScoreTotal) / float64(matchScoreCount)
				ratingCount++
			}
		}
	}

	if ratingCount > 0 {
		stats.AverageRating = totalRating / float64(ratingCount)
	}

	c.JSON(http.StatusOK, stats)
}

type WeeklyStarPlayer struct {
	PlayerID  string  `json:"playerId"`
	AvgRating float64 `json:"avgRating,omitempty"`
	Goals     int     `json:"goals,omitempty"`
	Assists   int     `json:"assists,omitempty"`
}

type WeeklyStars struct {
	HasMatches    bool              `json:"hasMatches"`
	PlayerOfWeek  *WeeklyStarPlayer `json:"playerOfWeek"`
	TopScorer     *WeeklyStarPlayer `json:"topScorer"`
	TopAssister   *WeeklyStarPlayer `json:"topAssister"`
}

func GetWeeklyStars(c *gin.Context) {
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

	now := time.Now().UTC()
	// ISO week: Monday = day 1
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	weekStart := now.AddDate(0, 0, -(weekday - 1)).Truncate(24 * time.Hour)
	weekEnd := weekStart.AddDate(0, 0, 7)

	var weekMatches []models.Match
	for _, m := range matches {
		if !m.Date.Before(weekStart) && m.Date.Before(weekEnd) {
			weekMatches = append(weekMatches, m)
		}
	}

	if len(weekMatches) == 0 {
		c.JSON(http.StatusOK, WeeklyStars{HasMatches: false})
		return
	}

	// Player of the Week: highest average rating
	type ratingAcc struct{ total, count int }
	ratingSums := map[primitive.ObjectID]*ratingAcc{}
	for _, m := range weekMatches {
		for _, r := range m.Ratings {
			for _, s := range r.Scores {
				if _, ok := ratingSums[s.PlayerID]; !ok {
					ratingSums[s.PlayerID] = &ratingAcc{}
				}
				ratingSums[s.PlayerID].total += s.Score
				ratingSums[s.PlayerID].count++
			}
		}
	}

	var playerOfWeek *WeeklyStarPlayer
	var bestAvg float64 = -1
	for pid, acc := range ratingSums {
		avg := float64(acc.total) / float64(acc.count)
		if avg > bestAvg {
			bestAvg = avg
			id := pid.Hex()
			playerOfWeek = &WeeklyStarPlayer{PlayerID: id, AvgRating: avg}
		}
	}

	// Top Scorer & Top Assister
	goalCounts := map[primitive.ObjectID]int{}
	assistCounts := map[primitive.ObjectID]int{}
	for _, m := range weekMatches {
		for _, ev := range m.Events {
			switch ev.Type {
			case "goal":
				goalCounts[ev.PlayerID]++
			case "assist":
				assistCounts[ev.PlayerID]++
			}
		}
	}

	var topScorer *WeeklyStarPlayer
	var maxGoals int
	for pid, goals := range goalCounts {
		if goals > maxGoals {
			maxGoals = goals
			id := pid.Hex()
			topScorer = &WeeklyStarPlayer{PlayerID: id, Goals: goals}
		}
	}

	var topAssister *WeeklyStarPlayer
	var maxAssists int
	for pid, assists := range assistCounts {
		if assists > maxAssists {
			maxAssists = assists
			id := pid.Hex()
			topAssister = &WeeklyStarPlayer{PlayerID: id, Assists: assists}
		}
	}

	c.JSON(http.StatusOK, WeeklyStars{
		HasMatches:   true,
		PlayerOfWeek: playerOfWeek,
		TopScorer:    topScorer,
		TopAssister:  topAssister,
	})
}
