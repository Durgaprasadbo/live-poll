package main

import (
	"log"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"pollapp/internal/auth"
	"pollapp/internal/config"
	"pollapp/internal/db"
	"pollapp/internal/handlers"
	"pollapp/internal/hub"
)

func main() {
	cfg := config.Load()

	mongoDB := db.ConnectMongo(cfg.MongoURI, cfg.MongoDBName)
	redisClient := db.ConnectRedis(cfg.RedisAddr, cfg.RedisPass)

	usersCol := mongoDB.Collection("users")
	pollsCol := mongoDB.Collection("polls")

	wsHub := hub.New(redisClient)

	authHandler := &handlers.AuthHandler{Users: usersCol, JWTSecret: cfg.JWTSecret}
	pollHandler := &handlers.PollHandler{Polls: pollsCol}
	voteHandler := &handlers.VoteHandler{Polls: pollsCol, Redis: redisClient}
	wsHandler := &handlers.WSHandler{Hub: wsHub}

	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{cfg.FrontendURL},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-Voter-Id"},
		AllowCredentials: true,
	}))

	r.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })

	api := r.Group("/api")
	{
		api.POST("/auth/signup", authHandler.Signup)
		api.POST("/auth/login", authHandler.Login)

		// Public: viewing + voting on a poll needs no login (link-sharing flow).
		api.GET("/polls/:id", pollHandler.GetPoll)
		api.GET("/polls/:id/results", voteHandler.Results)
		api.POST("/polls/:id/vote", voteHandler.Vote)

		// Protected: only a logged-in user can create/manage polls.
		protected := api.Group("")
		protected.Use(auth.RequireAuth(cfg.JWTSecret))
		{
			protected.POST("/polls", pollHandler.CreatePoll)
			protected.GET("/polls", pollHandler.MyPolls)
			protected.POST("/polls/:id/close", pollHandler.ClosePoll)
		}
	}

	r.GET("/ws/polls/:id", wsHandler.PollSocket)

	log.Printf("listening on :%s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
