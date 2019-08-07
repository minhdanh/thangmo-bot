package config

import (
	"flag"
	"github.com/go-redis/redis"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"log"
	"os"
	"strings"
)

type Config struct {
	TelegramApiToken      string
	TelegramChannel       string
	TelegramPreviewLink   bool
	BitLyEnabled          bool
	BitLyApiToken         string
	RedisClient           *redis.Client
	YcombinatorLink       bool
	MaxRssChannelsPerUser int
	Port                  int
	DatabaseURL           string
}

func NewConfig() *Config {
	configDir := ""

	flag.String("config-dir", "/etc/thangmo-bot", "Default config directory")
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()
	viper.BindPFlags(pflag.CommandLine)

	configDir = viper.GetString("config-dir")

	if configDir != "" {
		viper.AddConfigPath(configDir)
		log.Printf("Using config dir: %v", configDir)
	}

	viper.SetConfigName("config")
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Println("Error: config.yaml not found.")
		} else {
			log.Println(err)
		}
	}

	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	var config Config

	viper.SetDefault("Port", 3000)
	config.Port = viper.GetInt("port")

	// database
	config.DatabaseURL = viper.GetString("database_url")

	// telegram
	config.TelegramChannel = viper.GetString("telegram.channel")
	config.TelegramApiToken = viper.GetString("telegram.api_token")
	config.TelegramPreviewLink = viper.GetBool("telegram.preview_link")

	config.MaxRssChannelsPerUser = viper.GetInt("max_channels_per_user")

	// bitly
	config.BitLyEnabled = viper.GetBool("bitly.enabled")
	config.BitLyApiToken = viper.GetString("bitly.api_token")

	// redis
	redisCloudUrl := os.Getenv("REDISCLOUD_URL")
	redisOptions, err := redis.ParseURL(redisCloudUrl)
	if err == nil {
		log.Println("Using Redis config from REDISCLOUD_URL")
	}

	if redisOptions == nil {
		redisOptions = &redis.Options{
			Addr:     viper.GetString("redis.host") + ":" + viper.GetString("redis.port"),
			Password: viper.GetString("redis.password"),
			DB:       0,
		}
	}
	rc := redis.NewClient(redisOptions)
	config.RedisClient = rc

	return &config
}
