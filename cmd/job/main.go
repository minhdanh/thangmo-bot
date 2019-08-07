package main

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"log"
	"strconv"
	"time"

	"github.com/minhdanh/thangmo-bot/internal/bot"
	"github.com/minhdanh/thangmo-bot/internal/config"
	"github.com/minhdanh/thangmo/pkg/bitly"
	"github.com/minhdanh/thangmo/pkg/hackernews"
	"github.com/minhdanh/thangmo/pkg/telegram"

	"github.com/go-redis/redis"
	"github.com/jinzhu/gorm"
	"github.com/mmcdole/gofeed"
)

type ItemWrapper struct {
	Item            interface{}
	UserID          int
	Prefix          string
	RssLinkCheckSum string
}

func main() {
	config := config.NewConfig()

	var items []ItemWrapper

	redisClient := config.RedisClient

	db, err := gorm.Open("postgres", config.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Getting top stories from HackerNews")
	hnClient := hackernews.NewHNClient()
	hnItemIDs := hnClient.GetItemIDs()
	hnItems := make(map[int]*hackernews.HNItem, 500)

	// get HN subscribers
	var hnRegistrations []bot.HNRegistration
	db.Find(&hnRegistrations)
	log.Println(hnRegistrations)

	// get rss subscribers
	var rssRegistrations []bot.RSSRegistration
	db.Find(&rssRegistrations)

	for _, itemId := range hnItemIDs {
		var hnItem *hackernews.HNItem
		// get from memory first
		if item, ok := hnItems[itemId]; ok {
			hnItem = item
		} else {
			// if not exist in memory then get item from redis
			tmpHNItem, err := getHNItemFromRedis(redisClient, strconv.Itoa(itemId))
			if err != nil {
				// if not exist in redis, get from hackernews
				tmpHNItem = hnClient.GetItem(itemId)
				serialized, _ := json.Marshal(tmpHNItem)
				redisClient.Set(strconv.Itoa(itemId), string(serialized), 3600*time.Second)
				log.Println("Saved item to Redis: ", string(serialized))
			} else {
				log.Printf("Got item %v from Redis", itemId)
			}
			hnItem = tmpHNItem
			hnItems[itemId] = hnItem
		}

		for _, hnRegistration := range hnRegistrations {
			if checked, err := alreadyChecked(redisClient, hnRegistration.UserID, strconv.Itoa(itemId)); err != nil {
				log.Println(err)
				continue
			} else if checked {
				log.Printf("HackerNews item %v already checked for user %v", itemId, hnRegistration.UserID)
				continue
			}
			if hnItem.Score >= hnRegistration.MinScore {
				items = append(items, ItemWrapper{Item: hnItem, UserID: hnRegistration.UserID})
			} else {
				log.Printf("Item score is lower than %v", hnRegistration.MinScore)
			}
		}
	}

	for _, rssRegistration := range rssRegistrations {
		var rssLink bot.RSSLink
		var feed *gofeed.Feed
		db.Model(&rssRegistration).Related(&rssLink)
		log.Printf("Link: %v", rssLink.Url)

		log.Printf("Getting RSS content for %v", rssRegistration.Alias)
		fp := gofeed.NewParser()
		tmpFeed, err := getRssFeedFromRedis(redisClient, rssLink.Url)
		if err != nil {
			tmpFeed, _ = fp.ParseURL(rssLink.Url)
			serialized, _ := json.Marshal(tmpFeed)
			redisClient.Set(rssLink.Url, string(serialized), 3600*time.Second)
			log.Println("Saved Rss feed to Redis")
		}
		feed = tmpFeed

		if err != nil {
			log.Println(err)
			continue
		}

		log.Printf("RSS channel %v has %v items", rssRegistration.Alias, len(feed.Items))
		for _, item := range feed.Items {
			md5Sum := md5.Sum([]byte(item.Link))
			linkHash := hex.EncodeToString(md5Sum[:])
			if checked, err := alreadyChecked(redisClient, rssRegistration.UserID, linkHash); err != nil {
				log.Println(err)
				continue
			} else if checked {
				log.Printf("RSS item \"%v\" already checked for user %v", item.Title, rssRegistration.UserID)
				continue
			}
			items = append(items, ItemWrapper{Item: item, UserID: rssRegistration.UserID, Prefix: rssRegistration.Alias, RssLinkCheckSum: linkHash})
		}
	}

	log.Printf("Processing %v items", len(items))

	t := telegram.NewClient(config.TelegramApiToken, config.TelegramChannel, config.TelegramPreviewLink, config.YcombinatorLink)
	for _, item := range items {
		var url, redisKey string
		switch value := item.Item.(type) {
		case *hackernews.HNItem:
			log.Printf("Sending Telegram message, HackerNews item: %v", value.ID)
			url = value.URL
			redisKey = strconv.Itoa(value.ID)
		case *gofeed.Item:
			log.Printf("Sending Telegram message, RSS item: \"%v\"", value.Title)
			url = value.Link
			redisKey = item.RssLinkCheckSum
		}
		if config.BitLyEnabled {
			bitly := bitly.NewClient(config.BitLyApiToken)
			url = bitly.ShortenUrl(url)
		}
		_, err := t.SendMessageForItem(item.Item, url, item.Prefix, item.UserID)
		redisClient.HSet(strconv.Itoa(item.UserID), redisKey, "0")
		if err != nil {
			log.Println(err)
		}
	}
}

func alreadyChecked(redisClient *redis.Client, userId int, key string) (bool, error) {
	if _, err := redisClient.HGet(strconv.Itoa(userId), key).Result(); err == redis.Nil {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

func getHNItemFromRedis(redisClient *redis.Client, itemId string) (*hackernews.HNItem, error) {
	var item hackernews.HNItem

	if value, err := redisClient.Get(itemId).Result(); err == nil {

		if err := json.Unmarshal([]byte(value), &item); err != nil {
			return nil, err
		} else {
			return &item, nil
		}

	} else {
		return nil, err
	}
}

func getRssFeedFromRedis(redisClient *redis.Client, url string) (*gofeed.Feed, error) {
	var feed gofeed.Feed

	if value, err := redisClient.Get(url).Result(); err == nil {

		if err := json.Unmarshal([]byte(value), &feed); err != nil {
			return nil, err
		} else {
			return &feed, nil
		}

	} else {
		return nil, err
	}
}
