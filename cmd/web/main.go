package main

import (
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"github.com/jinzhu/gorm"
	"github.com/minhdanh/thangmo-bot/internal/bot"
	"github.com/minhdanh/thangmo-bot/internal/config"
	"github.com/minhdanh/thangmo-bot/internal/database"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithField("cmd", "thangmo-web")

func main() {
	config := config.NewConfig()

	port := strconv.Itoa(config.Port)
	if port == "" {
		log.WithField("PORT", port).Fatal("$PORT must be set")
	}

	db, err := gorm.Open("postgres", config.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}

	database.DBCon = db
	defer db.Close()

	bot := bot.NewBot(config)
	go bot.Start()

	templates := template.Must(template.ParseFiles("templates/index.html"))
	data := struct {
		TelegramChannel string
	}{
		strings.Replace(config.TelegramChannel, "@", "", -1),
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		l := log.WithField("path", r.URL.Path).WithField("method", r.Method)
		l.Println(r.RemoteAddr + " " + r.UserAgent())
		if err := templates.ExecuteTemplate(w, "index.html", data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			l.Println(err.Error())
		}
	})

	log.Println(http.ListenAndServe(":"+port, nil))
}
