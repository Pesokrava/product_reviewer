package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/Pesokrava/product_reviewer/internal/domain"
)

// RedisCache implements caching for products and reviews
type RedisCache struct {
	client           *redis.Client
	productRatingTTL time.Duration
	reviewsListTTL   time.Duration
}

// NewRedisCache creates a new Redis cache instance
func NewRedisCache(client *redis.Client, productRatingTTL, reviewsListTTL time.Duration) *RedisCache {
	return &RedisCache{
		client:           client,
		productRatingTTL: productRatingTTL,
		reviewsListTTL:   reviewsListTTL,
	}
}

// Product rating cache keys and methods

func (c *RedisCache) productRatingKey(productID uuid.UUID) string {
	return fmt.Sprintf("product:%s:rating", productID.String())
}

// GetProductRating retrieves cached product rating
func (c *RedisCache) GetProductRating(ctx context.Context, productID uuid.UUID) (float64, error) {
	key := c.productRatingKey(productID)
	val, err := c.client.Get(ctx, key).Float64()
	if err != nil {
		if err == redis.Nil {
			return 0, domain.ErrNotFound
		}
		return 0, err
	}
	return val, nil
}

// SetProductRating stores product rating in cache
func (c *RedisCache) SetProductRating(ctx context.Context, productID uuid.UUID, rating float64) error {
	key := c.productRatingKey(productID)
	return c.client.Set(ctx, key, rating, c.productRatingTTL).Err()
}

// InvalidateProductRating removes product rating from cache
func (c *RedisCache) InvalidateProductRating(ctx context.Context, productID uuid.UUID) error {
	key := c.productRatingKey(productID)
	return c.client.Del(ctx, key).Err()
}

// Product reviews list cache keys and methods

func (c *RedisCache) reviewsListKey(productID uuid.UUID, limit, offset int) string {
	return fmt.Sprintf("product:%s:reviews:limit:%d:offset:%d", productID.String(), limit, offset)
}

func (c *RedisCache) productCacheKeysSet(productID uuid.UUID) string {
	return fmt.Sprintf("product:%s:cache_keys", productID.String())
}

// GetReviewsList retrieves cached reviews list for a product
func (c *RedisCache) GetReviewsList(ctx context.Context, productID uuid.UUID, limit, offset int) ([]*domain.Review, error) {
	key := c.reviewsListKey(productID, limit, offset)
	val, err := c.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}

	var reviews []*domain.Review
	if err := json.Unmarshal([]byte(val), &reviews); err != nil {
		return nil, err
	}

	return reviews, nil
}

// SetReviewsList stores reviews list in cache and tracks the key in a SET
func (c *RedisCache) SetReviewsList(ctx context.Context, productID uuid.UUID, limit, offset int, reviews []*domain.Review) error {
	key := c.reviewsListKey(productID, limit, offset)
	trackingKey := c.productCacheKeysSet(productID)

	data, err := json.Marshal(reviews)
	if err != nil {
		return err
	}

	pipe := c.client.Pipeline()
	pipe.Set(ctx, key, data, c.reviewsListTTL)
	pipe.SAdd(ctx, trackingKey, key)
	pipe.Expire(ctx, trackingKey, c.reviewsListTTL)
	_, err = pipe.Exec(ctx)
	return err
}

// InvalidateReviewsList removes all cached review pages for a product using SET-based tracking
func (c *RedisCache) InvalidateReviewsList(ctx context.Context, productID uuid.UUID) error {
	trackingKey := c.productCacheKeysSet(productID)

	keys, err := c.client.SMembers(ctx, trackingKey).Result()
	if err != nil && err != redis.Nil {
		return err
	}

	if len(keys) > 0 {
		keys = append(keys, trackingKey)
		return c.client.Unlink(ctx, keys...).Err()
	}

	return nil
}

// InvalidateAllProductCache invalidates all cache entries for a product
func (c *RedisCache) InvalidateAllProductCache(ctx context.Context, productID uuid.UUID) error {
	if err := c.InvalidateProductRating(ctx, productID); err != nil && err != redis.Nil {
		return err
	}

	if err := c.InvalidateReviewsList(ctx, productID); err != nil && err != redis.Nil {
		return err
	}

	return nil
}
