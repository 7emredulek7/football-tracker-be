package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/user/7aside-tracker/handlers"
	"github.com/user/7aside-tracker/middleware"
)

func SetupRoutes(r *gin.Engine) {
	// Health check
	r.GET("/api/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	api := r.Group("/api")
	{
		// Public
		api.POST("/auth/login", handlers.Login)
		api.POST("/auth/register", handlers.RegisterWithInvitation)
		api.GET("/players", handlers.GetPlayers)
		api.GET("/matches", handlers.GetMatches)
		api.GET("/matches/:id", handlers.GetMatchByID)
		api.GET("/settings", handlers.GetSettings)
		api.GET("/stats/player/:id", handlers.GetPlayerStats)
		api.GET("/invitations/:token", handlers.GetInvitation)

		// Any authenticated user (owner or player)
		anyAuth := api.Group("/")
		anyAuth.Use(middleware.AnyAuthMiddleware())
		{
			anyAuth.POST("/matches/:id/ratings", handlers.AddRatings)
		}

		// Owner only
		owner := api.Group("/")
		owner.Use(middleware.AuthMiddleware())
		{
			owner.POST("/players", handlers.CreatePlayer)
			owner.PUT("/players/:id", handlers.UpdatePlayer)
			owner.DELETE("/players/:id", handlers.DeletePlayer)

			owner.POST("/matches", handlers.CreateMatch)
			owner.PUT("/matches/:id", handlers.UpdateMatch)
			owner.DELETE("/matches/:id", handlers.DeleteMatch)
			owner.POST("/matches/:id/events", handlers.AddEvents)

			owner.PUT("/settings/default-lineup", handlers.UpdateSettings)

			owner.POST("/invitations", handlers.CreateInvitation)
		}
	}
}
