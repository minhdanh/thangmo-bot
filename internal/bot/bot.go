package bot

import (
	"log"
	"strconv"
	"strings"

	"github.com/go-redis/redis"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/minhdanh/thangmo-bot/internal/config"
	"github.com/minhdanh/thangmo-bot/internal/database"
	"github.com/minhdanh/thangmo/pkg/telegram"

	_ "github.com/jinzhu/gorm/dialects/postgres"
)

type Bot struct {
	TelegramClient *telegram.TelegramClient
}

var (
	redisClient           *redis.Client
	maxRssChannelsPerUser int
)

func NewBot(config *config.Config) *Bot {
	var bot Bot
	t := telegram.NewClient(config.TelegramApiToken, config.TelegramChannel, config.TelegramPreviewLink, config.YcombinatorLink)
	bot.TelegramClient = t

	redisClient = config.RedisClient
	maxRssChannelsPerUser = config.MaxRssChannelsPerUser

	tables := []interface{}{&RSSLink{}, &HNRegistration{}, &RSSRegistration{}}
	for _, table := range tables {
		if !database.DBCon.HasTable(table) {
			log.Printf("Creating table %v", table)
			database.DBCon.CreateTable(table)
		}
		database.DBCon.AutoMigrate(table)
	}

	return &bot
}

func (b *Bot) Start() {
	stateChanged := map[int]bool{}

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := b.TelegramClient.TelegramBot.GetUpdatesChan(u)
	if err != nil {
		log.Fatal(err)
	}

	for update := range updates {
		if update.Message == nil {
			continue
		}

		userId := update.Message.From.ID
		incomingMessage := update.Message.Text
		log.Printf("[%v] %s", userId, incomingMessage)

		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
		if update.Message.IsCommand() {
			command := update.Message.Command()

			redisClient.HSet(strconv.Itoa(userId), "current_state", command)
			arguments := strings.Split(update.Message.CommandArguments(), " ")
			switch command {
			case "start":
				msg.Text = commandHelpMessages["start"]
				redisClient.HSet(strconv.Itoa(userId), "current_state", "hackernews")
				redisClient.HSet(strconv.Itoa(userId), "current_step", "begin")
				stateChanged[userId] = true
			case "hackernews":
				switch strings.ToLower(arguments[0]) {
				case "enable":
					if len(arguments) == 2 {
						minimumScore, err := validateMinimumScore(arguments[1])
						if err != nil {
							msg.Text = err.Error()
						} else {
							if _, err := createOrUpdateHNRegistration(userId, minimumScore); err != nil {
								log.Println(err)
							}
							msg.Text = "Enabled HackerNews with minimum score of " + arguments[1]
						}
					} else {
						msg.Text = "The command you entered is not what I expect.\n\n"
						msg.Text += commandHelpMessages["hackernews"]
					}
				case "disable":
					if _, err := deleteHNRegistration(userId); err != nil {
						msg.Text = err.Error()
					} else {
						msg.Text = "Ok, I disabled HackerNews for you."
					}
				default:
					msg.Text = commandHelpMessages["hackernews"]
				}
			case "rss":

				switch strings.ToLower(arguments[0]) {
				case "add":
					argumentsLen := len(arguments)
					if argumentsLen >= 3 {
						channelURL := arguments[argumentsLen-1]
						channelName := strings.Join(arguments[1:argumentsLen-1], " ")
						if _, err := checkMaximumChannels(userId); err != nil {
							msg.Text = err.Error()
						} else if _, err := validateChannelName(channelName); err != nil {
							msg.Text = err.Error()
						} else if _, err := validateChannelURL(channelURL); err != nil {
							msg.Text = err.Error()
						} else if _, err := addRSSRegistration(userId, channelName, channelURL); err != nil {
							msg.Text = err.Error()
						} else {
							msg.Text = "Ok, I saved your channel " + channelName + "."
						}
					} else {
						msg.Text = "The command you entered is not what I expect.\n\n"
						msg.Text += commandHelpMessages["rss"]
					}
				case "remove":
					if len(arguments) >= 2 {
						alias := strings.Join(arguments[1:], " ")
						if _, err := deleteRSSRegistration(userId, alias); err != nil {
							msg.Text = "Oops! I couldn't remove your channel. Please make sure you gave me the right name. You can check your current channels with `/rss list`."
						} else {
							msg.Text = "Ok, I removed your channel. Check your current channels with `/rss list`."
						}
					} else {
						msg.Text = "The command you entered is not what I expect.\n\n"
						msg.Text += commandHelpMessages["rss"]
					}
				case "list":
					channels, err := getRSSChannels(userId)
					if err != nil {
						log.Println(err)
					}
					if len(channels) > 0 {
						msg.Text = "Channels you're subscribing:\n"
						for _, c := range channels {
							var rssLink RSSLink
							database.DBCon.Model(&c).Related(&rssLink)
							msg.Text += "- " + c.Alias + " " + rssLink.Url + "\n"
						}
					} else {
						msg.Text = "You are not subscribing to any channels. Why not add some?"
					}
				default:
					msg.Text = commandHelpMessages["rss"]
				}
			case "cancel":
				msg.Text = "Action cancelled. I have nothing to do now."
			case "help":
				msg.Text = commandHelpMessages["start"]
				msg.Text += "\n\n*HackerNews*"
				msg.Text += commandHelpMessages["hackernews"]
				msg.Text += "\n\n*RSS*"
				msg.Text += commandHelpMessages["rss"]
				msg.ParseMode = "Markdown"
			default:
				msg.Text = "Hmm. I'm not sure what that command is. Try /help to understand me better."
			}
		} else {
			currentState := redisClient.HGet(strconv.Itoa(userId), "current_state").Val()
			log.Printf("currentState: %s", currentState)
			switch currentState {
			case "hackernews", "rss":
				currentStep := redisClient.HGet(strconv.Itoa(userId), "current_step").Val()
				callbackFunc := currentState + "_" + currentStep
				log.Printf("Callback function: %v", callbackFunc)
				ok, message, nextState, nextStep := handlerFuncMap[callbackFunc](userId, incomingMessage)
				if ok {
					if nextStep == "" {
						if nextState != "" {
							redisClient.HSet(strconv.Itoa(userId), "current_state", nextState)
							redisClient.HSet(strconv.Itoa(userId), "current_step", "begin")
							stateChanged[userId] = true
						} else {
							// do nothing
						}
					} else {
						redisClient.HSet(strconv.Itoa(userId), "current_step", nextStep)
					}
				}
				msg.Text = message
			default:
				msg.Text = "You know, I'm just a bot and I have very limitted understanding of human language. As a result I'm not quite sure what you want. Until the day when I can master the languages of human, why not try `/help` command to see what I can do?"
			}
		}
		b.TelegramClient.TelegramBot.Send(msg)
		if stateChanged[userId] {
			newState := redisClient.HGet(strconv.Itoa(userId), "current_state").Val()
			log.Printf("State changed. New state: %v", newState)

			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
			msg.Text = newStateMessages[newState]
			log.Println(msg.Text)
			b.TelegramClient.TelegramBot.Send(msg)

			stateChanged[userId] = false
		}
	}
}
