package handlers

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"pollapp/internal/models"
)

type PollHandler struct {
	Polls *mongo.Collection
}

type createPollReq struct {
	Question string   `json:"question"`
	Options  []string `json:"options"`
}

// CreatePoll requires auth (see router). This enforces the brief's
// "poll creation shouldn't be wide open to anyone with the URL" rule.
func (h *PollHandler) CreatePoll(c *gin.Context) {
	var req createPollReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	req.Question = strings.TrimSpace(req.Question)
	if req.Question == "" || len(req.Question) > 300 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "question is required (max 300 chars)"})
		return
	}

	cleaned := make([]models.Option, 0, len(req.Options))
	seen := map[string]bool{}
	for _, raw := range req.Options {
		t := strings.TrimSpace(raw)
		if t == "" || len(t) > 150 {
			continue
		}
		key := strings.ToLower(t)
		if seen[key] {
			continue
		}
		seen[key] = true
		cleaned = append(cleaned, models.Option{ID: uuid.NewString(), Text: t})
	}
	if len(cleaned) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at least 2 distinct, non-empty options are required"})
		return
	}
	if len(cleaned) > 10 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at most 10 options are allowed"})
		return
	}

	userIDHex := c.GetString("user_id")
	ownerID, err := primitive.ObjectIDFromHex(userIDHex)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user"})
		return
	}

	poll := models.Poll{
		ID:        primitive.NewObjectID(),
		Question:  req.Question,
		Options:   cleaned,
		OwnerID:   ownerID,
		IsActive:  true,
		CreatedAt: time.Now(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := h.Polls.InsertOne(ctx, poll); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create poll"})
		return
	}

	c.JSON(http.StatusCreated, poll)
}

// GetPoll is public — anyone with the link can view/vote on a poll.
func (h *PollHandler) GetPoll(c *gin.Context) {
	id := c.Param("id")
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid poll id"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var poll models.Poll
	if err := h.Polls.FindOne(ctx, bson.M{"_id": objID}).Decode(&poll); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "poll not found"})
		return
	}

	c.JSON(http.StatusOK, poll)
}

// MyPolls lists polls owned by the authenticated user (dashboard).
func (h *PollHandler) MyPolls(c *gin.Context) {
	userIDHex := c.GetString("user_id")
	ownerID, err := primitive.ObjectIDFromHex(userIDHex)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cur, err := h.Polls.Find(ctx, bson.M{"owner_id": ownerID})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not fetch polls"})
		return
	}
	defer cur.Close(ctx)

	polls := make([]models.Poll, 0)
	if err := cur.All(ctx, &polls); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not decode polls"})
		return
	}

	c.JSON(http.StatusOK, polls)
}

// ClosePoll lets the owner stop accepting new votes.
func (h *PollHandler) ClosePoll(c *gin.Context) {
	id := c.Param("id")
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid poll id"})
		return
	}
	userIDHex := c.GetString("user_id")
	ownerID, _ := primitive.ObjectIDFromHex(userIDHex)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := h.Polls.UpdateOne(ctx,
		bson.M{"_id": objID, "owner_id": ownerID},
		bson.M{"$set": bson.M{"is_active": false}},
	)
	if err != nil || res.MatchedCount == 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "poll not found or not owned by you"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "closed"})
}
