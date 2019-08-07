package bot

import (
	"log"
	"strconv"
	"strings"
)

var (
	handlerFuncMap = map[string]func(int, string) (bool, string, string, string){
		"hackernews_begin": func(userId int, value string) (ok bool, message string, nextState string, nextStep string) {
			s := strings.ToLower(value)
			switch s {
			case "yes":
				redisClient.HSet(strconv.Itoa(userId), "hackernews_enabled", true)
				message := "Minimum points of HackNews items?"
				nextStep := "minimum_points"
				return true, message, "", nextStep
			case "no":
				redisClient.HSet(strconv.Itoa(userId), "hackernews_enabled", false)
				message = "Ok. You don't want to receive news from HackerNews."
				nextState = "rss"
				return true, message, nextState, ""
			default:
				message = "Please choose `Yes` or `No`."
				return false, message, "", ""
			}
		},
		"hackernews_minimum_points": func(userId int, value string) (ok bool, message string, nextState string, nextStep string) {
			point, err := validateMinimumScore(value)
			if err != nil {
				message := err.Error()
				return false, message, "", ""
			}

			redisClient.HDel(strconv.Itoa(userId), "hackernews_enabled")

			if _, err := createOrUpdateHNRegistration(userId, point); err != nil {
				log.Println(err)
			}

			message = "Ok. You want to be notified items with " + value + " points up."
			nextState = "rss"
			return true, message, nextState, ""
		},
		"rss_begin": func(userId int, value string) (ok bool, message string, nextState string, nextStep string) {
			s := strings.ToLower(value)
			switch s {
			case "yes":
				if _, err := checkMaximumChannels(userId); err != nil {
					message := err.Error()
					return false, message, "finished", ""
				} else {
					message := "Great. Let's add your favourite RSS channel. Remember you can add up to " + strconv.Itoa(maxRssChannelsPerUser) + " channels.\nWhat is the name of the RSS channel?"
					return true, message, "", "channel_name"
				}
			case "no":
				message := "Ok. You can change that later with `/rss add` command."
				return true, message, "finished", ""
			default:
				message := "Please choose `Yes` or `No`"
				return false, message, "", ""
			}
		},
		"rss_channel_name": func(userId int, value string) (ok bool, message string, nextState string, nextStep string) {
			if _, err := validateChannelName(value); err != nil {
				message := err.Error()
				return false, message, "", ""
			} else {
				// TODO: use step new message instead of including in current step
				redisClient.HSet(strconv.Itoa(userId), "rss_channel_name", value)
				message := "Your RSS channel name is " + value + ". What is the URL of the channel?"
				return true, message, "", "channel_url"
			}
		},
		"rss_channel_url": func(userId int, value string) (ok bool, message string, nextState string, nextStep string) {
			channelName := redisClient.HGet(strconv.Itoa(userId), "rss_channel_name").Val()
			if _, err := validateChannelURL(value); err != nil {
				message := err.Error()
				return false, message, "", ""
			}
			if _, err := addRSSRegistration(userId, channelName, value); err != nil {
				message := err.Error()
				return false, message, "", ""
			} else {
				redisClient.HDel(strconv.Itoa(userId), "rss_channel_name")
				message := "Your RSS channel URL is " + value + "\nDo you want to add more channel?"
				return true, message, "", "begin"
			}
		},
		"finished": func(userId int, value string) (ok bool, message string, nextState string, nextStep string) {
			return true, "", "", ""
		},
	}
)
