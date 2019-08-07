module github.com/minhdanh/thangmo-bot

go 1.12

require (
	github.com/PuerkitoBio/goquery v1.5.0 // indirect
	github.com/go-redis/redis v6.15.2+incompatible
	github.com/go-telegram-bot-api/telegram-bot-api v4.6.4+incompatible
	github.com/jinzhu/gorm v1.9.10
	github.com/minhdanh/thangmo v0.0.0-20190807115922-4eae8961b2a3
	github.com/mmcdole/gofeed v1.0.0-beta2
	github.com/mmcdole/goxpp v0.0.0-20181012175147-0068e33feabf // indirect
	github.com/pkg/errors v0.8.0
	github.com/sirupsen/logrus v1.4.2
	github.com/spf13/pflag v1.0.3
	github.com/spf13/viper v1.4.0
	github.com/ungerik/go-rss v0.0.0-20190314071843-19c5ce3f500c
)

replace github.com/minhdanh/thangmo => ../thangmo
