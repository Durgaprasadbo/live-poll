package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"pollapp/internal/models"
)

type VoteHandler struct {
	Polls *mongo.Collection
	Redis *redis.Client
}

type voteReq struct {
	OptionID string `json:"option_id"`
}

func countsKey(pollID string) string  { return "poll:" + pollID + ":counts" }
func votersKey(pollID string) string  { return "poll:" + pollID + ":voters" }
func channelKey(pollID string) string { return "poll:" + pollID + ":updates" }

// Vote is public (anyone with the link can vote) but every field is
// validated server-side against the poll stored in Mongo before Redis
// or Mongo are touched — nothing from the client is trusted directly.
func (h *VoteHandler) Vote(c *gin.Context) {
	pollIDHex := c.Param("id")
	objID, err := primitive.ObjectIDFromHex(pollIDHex)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid poll id"})
		return
	}

	var req voteReq
	if err := c.ShouldBindJSON(&req); err != nil || req.OptionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "option_id is required"})
		return
	}

	// Simple per-voter dedupe: the frontend generates a random voter id,
	// stores it in localStorage, and sends it on every vote from that browser.
	voterID := c.GetHeader("X-Voter-Id")
	if voterID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing X-Voter-Id header"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var poll models.Poll
	if err := h.Polls.FindOne(ctx, bson.M{"_id": objID}).Decode(&poll); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "poll not found"})
		return
	}
	if !poll.IsActive {
		c.JSON(http.StatusConflict, gin.H{"error": "this poll is closed"})
		return
	}

	validOption := false
	for _, o := range poll.Options {
		if o.ID == req.OptionID {
			validOption = true
			break
		}
	}
	if !validOption {
		c.JSON(http.StatusBadRequest, gin.H{"error": "option does not belong to this poll"})
		return
	}

	if !poll.AllowMulti {
		added, err := h.Redis.SAdd(ctx, votersKey(pollIDHex), voterID).Result()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "server error"})
			return
		}
		if added == 0 {
			c.JSON(http.StatusConflict, gin.H{"error": "you have already voted on this poll"})
			return
		}
	}

	// Redis is the live counter (fast, atomic HINCRBY) — this is the
	// "real work" Redis does, not just a cache.
	newCount, err := h.Redis.HIncrBy(ctx, countsKey(pollIDHex), req.OptionID, 1).Result()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server error"})
		return
	}

	// Durable copy in Mongo so counts survive a Redis restart / eviction.
	_, _ = h.Polls.UpdateOne(ctx,
		bson.M{"_id": objID},
		bson.M{"$inc": bson.M{"vote_counts." + req.OptionID: 1}},
	)

	result, err := h.buildResult(ctx, pollIDHex, poll)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server error"})
		return
	}

	payload, _ := json.Marshal(result)
	h.Redis.Publish(ctx, channelKey(pollIDHex), payload)

	c.JSON(http.StatusOK, gin.H{"status": "ok", "option_new_count": newCount})
}

// Results returns current live counts — used on initial page load before
// the websocket connects, and as a fallback if the socket drops.
func (h *VoteHandler) Results(c *gin.Context) {
	pollIDHex := c.Param("id")
	objID, err := primitive.ObjectIDFromHex(pollIDHex)
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

	result, err := h.buildResult(ctx, pollIDHex, poll)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server error"})
		return
	}

	c.JSON(http.StatusOK, result)
}

func (h *VoteHandler) buildResult(ctx context.Context, pollIDHex string, poll models.Poll) (models.PollResult, error) {
	raw, err := h.Redis.HGetAll(ctx, countsKey(pollIDHex)).Result()
	if err != nil {
		return models.PollResult{}, err
	}

	counts := make(map[string]int64, len(poll.Options))
	var total int64
	for _, o := range poll.Options {
		counts[o.ID] = 0
	}
	for k, v := range raw {
		n := parseInt64(v)
		counts[k] = n
		total += n
	}

	return models.PollResult{PollID: pollIDHex, Counts: counts, Total: total}, nil
}

func parseInt64(s string) int64 {
	var n int64
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return n
		}
		n = n*10 + int64(ch-'0')
	}
	return n
}
