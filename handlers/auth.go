package handlers

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/user/7aside-tracker/config"
	"github.com/user/7aside-tracker/models"
	"github.com/user/7aside-tracker/utils"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/crypto/bcrypt"
)

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

func Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	collection := config.DB.Collection("users")
	var user models.User
	err := collection.FindOne(context.Background(), bson.M{"email": req.Email}).Decode(&user)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}

	playerIDHex := ""
	if user.PlayerID != nil {
		playerIDHex = user.PlayerID.Hex()
	}
	token, err := utils.GenerateToken(user.ID.Hex(), user.Role, playerIDHex)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": token, "role": user.Role})
}

func LinkOwnerToPlayer(c *gin.Context) {
	playerIDParam := c.Param("playerId")
	playerObjID, err := primitive.ObjectIDFromHex(playerIDParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid player ID"})
		return
	}

	userIDHex, _ := c.Get("userId")
	ownerObjID, err := primitive.ObjectIDFromHex(userIDHex.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user context"})
		return
	}

	ctx := context.Background()
	players := config.DB.Collection("players")
	users := config.DB.Collection("users")

	// Find the target player
	var player models.Player
	if err := players.FindOne(ctx, bson.M{"_id": playerObjID}).Decode(&player); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Player not found"})
		return
	}

	// Ensure the player is not already linked to a different user
	if player.UserID != nil && *player.UserID != ownerObjID {
		c.JSON(http.StatusConflict, gin.H{"error": "Player is already linked to another account"})
		return
	}

	// Find the current owner user
	var owner models.User
	if err := users.FindOne(ctx, bson.M{"_id": ownerObjID}).Decode(&owner); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user"})
		return
	}

	// Ensure no other user is already linked as owner to any player (only one owner-player link)
	var existingLinked models.User
	err = users.FindOne(ctx, bson.M{
		"playerId": playerObjID,
		"_id":      bson.M{"$ne": ownerObjID},
	}).Decode(&existingLinked)
	if err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Player is already linked to another account"})
		return
	}

	// If owner was previously linked to a different player, clear that player's userId
	if owner.PlayerID != nil && *owner.PlayerID != playerObjID {
		_, _ = players.UpdateOne(ctx,
			bson.M{"_id": owner.PlayerID},
			bson.M{"$unset": bson.M{"userId": ""}},
		)
	}

	// Link player -> owner
	_, err = players.UpdateOne(ctx,
		bson.M{"_id": playerObjID},
		bson.M{"$set": bson.M{"userId": ownerObjID}},
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update player"})
		return
	}

	// Link owner -> player
	_, err = users.UpdateOne(ctx,
		bson.M{"_id": ownerObjID},
		bson.M{"$set": bson.M{"playerId": playerObjID}},
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
		return
	}

	token, err := utils.GenerateToken(ownerObjID.Hex(), "owner", playerObjID.Hex())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": token, "role": "owner", "playerId": playerObjID.Hex()})
}
