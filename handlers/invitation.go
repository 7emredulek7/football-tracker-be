package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/user/7aside-tracker/config"
	"github.com/user/7aside-tracker/models"
	"github.com/user/7aside-tracker/utils"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/crypto/bcrypt"
)

func CreateInvitation(c *gin.Context) {
	var req struct {
		PlayerID string `json:"playerId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	playerObjID, err := primitive.ObjectIDFromHex(req.PlayerID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid player ID"})
		return
	}

	playersColl := config.DB.Collection("players")
	var player models.Player
	if err := playersColl.FindOne(context.Background(), bson.M{"_id": playerObjID}).Decode(&player); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Player not found"})
		return
	}

	if player.IsGuest {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot invite a guest player"})
		return
	}

	if player.UserID != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Player already has an account"})
		return
	}

	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}
	tokenStr := hex.EncodeToString(tokenBytes)

	invitation := models.Invitation{
		ID:        primitive.NewObjectID(),
		PlayerID:  playerObjID,
		Token:     tokenStr,
		Used:      false,
		ExpiresAt: time.Now().Add(72 * time.Hour),
		CreatedAt: time.Now(),
	}

	invColl := config.DB.Collection("invitations")
	if _, err := invColl.InsertOne(context.Background(), invitation); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create invitation"})
		return
	}

	frontendURL := strings.TrimRight(os.Getenv("FRONTEND_URL"), "/")
	inviteURL := frontendURL + "/register?token=" + tokenStr

	c.JSON(http.StatusCreated, gin.H{
		"token": tokenStr,
		"url":   inviteURL,
	})
}

func GetInvitation(c *gin.Context) {
	tokenStr := c.Param("token")

	invColl := config.DB.Collection("invitations")
	var invitation models.Invitation
	if err := invColl.FindOne(context.Background(), bson.M{"token": tokenStr}).Decode(&invitation); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Invitation not found"})
		return
	}

	if invitation.Used {
		c.JSON(http.StatusGone, gin.H{"error": "Invitation already used"})
		return
	}

	if time.Now().After(invitation.ExpiresAt) {
		c.JSON(http.StatusGone, gin.H{"error": "Invitation has expired"})
		return
	}

	playersColl := config.DB.Collection("players")
	var player models.Player
	if err := playersColl.FindOne(context.Background(), bson.M{"_id": invitation.PlayerID}).Decode(&player); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to find player"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"playerFirstName": player.FirstName,
		"playerLastName":  player.LastName,
		"playerId":        player.ID.Hex(),
	})
}

type RegisterRequest struct {
	Token    string `json:"token" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

func RegisterWithInvitation(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	invColl := config.DB.Collection("invitations")
	var invitation models.Invitation
	if err := invColl.FindOne(context.Background(), bson.M{"token": req.Token}).Decode(&invitation); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Invitation not found"})
		return
	}

	if invitation.Used {
		c.JSON(http.StatusGone, gin.H{"error": "Invitation already used"})
		return
	}

	if time.Now().After(invitation.ExpiresAt) {
		c.JSON(http.StatusGone, gin.H{"error": "Invitation has expired"})
		return
	}

	usersColl := config.DB.Collection("users")
	count, err := usersColl.CountDocuments(context.Background(), bson.M{"email": req.Email})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check email"})
		return
	}
	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "Email already in use"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	playerIDCopy := invitation.PlayerID
	user := models.User{
		ID:           primitive.NewObjectID(),
		Email:        req.Email,
		PasswordHash: string(hash),
		Role:         "player",
		PlayerID:     &playerIDCopy,
		CreatedAt:    time.Now(),
	}

	if _, err := usersColl.InsertOne(context.Background(), user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	playersColl := config.DB.Collection("players")
	_, _ = playersColl.UpdateOne(
		context.Background(),
		bson.M{"_id": invitation.PlayerID},
		bson.M{"$set": bson.M{"userId": user.ID}},
	)

	_, _ = invColl.UpdateOne(
		context.Background(),
		bson.M{"_id": invitation.ID},
		bson.M{"$set": bson.M{"used": true}},
	)

	jwtToken, err := utils.GenerateToken(user.ID.Hex(), "player", invitation.PlayerID.Hex())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"token": jwtToken,
		"role":  "player",
	})
}
