package services

import (
	"context"
	"fmt"
	"time"

	"github.com/divyam234/teldrive/internal/cache"
	"github.com/divyam234/teldrive/internal/crypt"
	"github.com/divyam234/teldrive/internal/tgc"
	"github.com/divyam234/teldrive/pkg/models"
	"github.com/divyam234/teldrive/pkg/schemas"
	"github.com/divyam234/teldrive/pkg/types"
	"github.com/gotd/td/tg"
	"github.com/pkg/errors"
	"gorm.io/gorm"
)

func getParts(ctx context.Context, client *tg.Client, cache cache.Cacher, file *schemas.FileOutFull, userID string) ([]types.Part, error) {

	parts := []types.Part{}

	key := fmt.Sprintf("files:messages:%s:%s", file.Id, userID)

	err := cache.Get(key, &parts)

	if err == nil {
		return parts, nil
	}

	ids := []int{}
	for _, part := range file.Parts {
		ids = append(ids, int(part.ID))
	}
	messages, err := tgc.GetMessages(ctx, client, ids, file.ChannelID)

	if err != nil {
		return nil, err
	}

	for i, message := range messages {
		item := message.(*tg.Message)
		media := item.Media.(*tg.MessageMediaDocument)
		document := media.Document.(*tg.Document)

		part := types.Part{
			ID:   file.Parts[i].ID,
			Size: document.Size,
			Salt: file.Parts[i].Salt,
		}
		if file.Encrypted {
			part.DecryptedSize, _ = crypt.DecryptedSize(document.Size)
		}
		parts = append(parts, part)
	}
	cache.Set(key, &parts, 60*time.Minute)
	return parts, nil
}

func getDefaultChannel(db *gorm.DB, cache cache.Cacher, userID int64) (int64, error) {

	var channelId int64
	key := fmt.Sprintf("users:channel:%d", userID)

	err := cache.Get(key, &channelId)

	if err == nil {
		return channelId, nil
	}

	var channelIds []int64
	db.Model(&models.Channel{}).Where("user_id = ?", userID).Where("selected = ?", true).
		Pluck("channel_id", &channelIds)

	if len(channelIds) == 1 {
		channelId = channelIds[0]
		cache.Set(key, channelId, 0)
	}

	if channelId == 0 {
		return channelId, errors.New("default channel not set")
	}

	return channelId, nil
}

func getBotsToken(db *gorm.DB, cache cache.Cacher, userID, channelId int64) ([]string, error) {
	var bots []string

	key := fmt.Sprintf("users:bots:%d:%d", userID, channelId)

	err := cache.Get(key, &bots)

	if err == nil {
		return bots, nil
	}

	if err := db.Model(&models.Bot{}).Where("user_id = ?", userID).
		Where("channel_id = ?", channelId).Pluck("token", &bots).Error; err != nil {
		return nil, err
	}

	cache.Set(key, &bots, 0)
	return bots, nil

}
