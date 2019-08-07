package bot

import (
	"errors"
	"log"
	"strconv"

	"github.com/minhdanh/thangmo-bot/internal/database"

	"github.com/jinzhu/gorm"
	"github.com/mmcdole/gofeed"
)

func validateMinimumScore(value string) (point int, err error) {
	minScore, err := strconv.Atoi(value)
	if err != nil {
		return -1, errors.New("Point should be a number.")
	}
	if minScore > 500 || minScore < 1 {
		return -1, errors.New("Point should be within 1 to 500.")
	}
	return minScore, nil
}

func createOrUpdateHNRegistration(userId int, point int) (ok bool, err error) {
	hnRegistration := HNRegistration{}
	log.Printf("User id: %v", userId)
	if r := database.DBCon.Where(&HNRegistration{UserID: userId}).First(&hnRegistration); r.Error != nil && !gorm.IsRecordNotFoundError(r.Error) {
		return false, r.Error
	}
	if hnRegistration.ID == 0 {
		log.Printf("Creating HNRegistration. User: %v, MinScore: %v", userId, point)
		if r := database.DBCon.Create(&HNRegistration{UserID: userId, MinScore: point}); r.Error != nil {
			return false, r.Error
		}
	} else {
		log.Printf("Updating HNRegistration. User: %v, MinScore: %v", userId, point)
		hnRegistration.MinScore = point
		if r := database.DBCon.Save(&hnRegistration); r.Error != nil {
			return false, r.Error
		}
	}
	return true, nil
}

func getRSSChannels(userId int) (rssRegistrations []RSSRegistration, err error) {
	var registrations []RSSRegistration
	database.DBCon.Where("user_id = ?", userId).Find(&registrations)
	return registrations, nil
}
func checkMaximumChannels(userId int) (ok bool, err error) {
	var count int
	database.DBCon.Model(&RSSRegistration{}).Where("user_id = ?", userId).Count(&count)
	if count >= maxRssChannelsPerUser {
		return false, errors.New("Sorry, you have reached the maximum RSS channels you can add.\nYou can see your existing channels by using command `/rss list`, then remove a channel with `/rss rm` command if you wish.")
	}
	return true, nil
}

func validateChannelName(channelName string) (ok bool, err error) {
	if channelName == "" || len(channelName) > 64 {
		return false, errors.New("Channel name is not valid. It should not be empty and not longer than 64 characters.")
	}
	return true, nil
}

func validateChannelURL(channelURL string) (ok bool, err error) {
	fp := gofeed.NewParser()
	_, err = fp.ParseURL(channelURL)
	if err != nil {
		return false, errors.New("Channel URL is not valid. Please make sure the URL has the right format and it's returning XML content.")
	}
	return true, nil
}

func addRSSRegistration(userId int, channelName string, channelURL string) (ok bool, err error) {
	rssLink := RSSLink{}
	if r := database.DBCon.Where("url = ?", channelURL).First(&rssLink); r.Error != nil {
		log.Println(r.Error)
	}

	if rssLink.ID == 0 {
		log.Printf("Creating RSSLink: %v", channelURL)
		rssLink.AddedBy = userId
		rssLink.Url = channelURL
		if r := database.DBCon.Create(&rssLink); r.Error != nil {
			log.Println(r.Error)
		}
	}

	rssRegistration := RSSRegistration{}
	if r := database.DBCon.Where("rss_link_id = ? AND user_id = ?", rssLink.ID, userId).First(&rssRegistration); r.Error != nil {
		log.Println(r.Error)
	}
	if rssRegistration.RSSLinkID > 0 {
		return false, errors.New("You already added that channel URL with the name `" + rssRegistration.Alias + "`. Please choose another URL or `/cancel` to cancel creating this channel.")
	}

	log.Printf("Creating RSSRegistration for user %v", userId)
	log.Printf("RSSLinkID: %v", rssLink.ID)
	if r := database.DBCon.Create(&RSSRegistration{RSSLinkID: rssLink.ID, UserID: userId, Alias: channelName}); r.Error != nil {
		log.Println(r.Error)
	}
	return true, nil
}

func deleteHNRegistration(userId int) (ok bool, err error) {
	hnRegistration := HNRegistration{}
	if r := database.DBCon.Where("user_id = ?", userId).First(&hnRegistration); r.Error != nil {
		log.Println(r.Error)
		return false, errors.New("User has not enabled HackerNews.")
	}
	if hnRegistration.ID > 0 {
		database.DBCon.Unscoped().Delete(&hnRegistration)
	}
	return true, nil
}

func deleteRSSRegistration(userId int, alias string) (ok bool, err error) {
	rssRegistration := RSSRegistration{}
	if r := database.DBCon.Where("alias = ? AND user_id = ?", alias, userId).First(&rssRegistration); r.Error != nil {
		log.Println(r.Error)
		return false, errors.New("There's no channel with such a name.")
	}
	if rssRegistration.RSSLinkID > 0 {
		database.DBCon.Unscoped().Delete(&rssRegistration)
	}
	return true, nil
}
