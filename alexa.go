package main

import (
	"database/sql"
	"fmt"
	"github.com/brianglass/orthocal"
	"github.com/gorilla/mux"
	alexa "github.com/mikeflynn/go-alexa/skillserver"
	"regexp"
	"time"
)

var verseRe = regexp.MustCompile(`<.*?>`)

type Skill struct {
	db        *sql.DB
	bible     *orthocal.Bible
	useJulian bool
	doJump    bool
}

func NewSkill(router *mux.Router, appid string, db *sql.DB, useJulian, doJump bool, bible *orthocal.Bible) *Skill {
	var skill Skill

	skill.db = db
	skill.bible = bible
	skill.useJulian = useJulian
	skill.doJump = doJump

	apps := map[string]interface{}{
		"/echo/": alexa.EchoApplication{
			AppID:    appid,
			OnIntent: skill.intentHandler,
			OnLaunch: skill.launchHandler,
		},
	}

	alexa.Init(apps, router)

	return &skill
}

func (self *Skill) launchHandler(request *alexa.EchoRequest, response *alexa.EchoResponse) {
	var text, card string

	today := time.Now().In(TZ)

	factory := orthocal.NewDayFactory(self.useJulian, self.doJump, self.db)
	day := factory.NewDay(today.Year(), int(today.Month()), today.Day(), self.bible)

	text = "Today is the " + day.Titles[0] + ". "
	text += FastingStatement(day) + " "
	text += "The reading is from " + day.Readings[0].Display + ". "

	card = text

	var passage string
	for _, verse := range day.Readings[0].Passage {
		passage += verseRe.ReplaceAllString(verse.Content, "") + " "
	}

	response.OutputSpeech(text+passage).Card("Orthodox Daily", card)
}

func (self *Skill) intentHandler(request *alexa.EchoRequest, response *alexa.EchoResponse) {
	factory := orthocal.NewDayFactory(self.useJulian, self.doJump, self.db)

	switch request.GetIntentName() {
	case "FastingIntent":
		var date time.Time

		if when, e := request.GetSlotValue("date"); e == nil && len(when) > 0 {
			date, e = time.ParseInLocation("2006-01-02", when, TZ)
			if e != nil {
				response.OutputSpeech("I didn't understand the date you requested.")
				return
			}
		} else {
			now := time.Now().In(TZ)
			date = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, TZ)
		}

		day := factory.NewDay(date.Year(), int(date.Month()), date.Day(), nil)
		text := FastingStatement(day)

		response.OutputSpeech(text).Card("Orthodox Daily", text)
	}
}

func FastingStatement(day *orthocal.Day) string {
	var text, when string

	now := time.Now().In(TZ)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, TZ)
	date := time.Date(day.Year, time.Month(day.Month), day.Day, 0, 0, 0, 0, TZ)

	hours := date.Sub(today).Hours()
	if 0 <= hours && hours < 24 {
		when = "Today, " + date.Format("January 2")
	} else if 24 <= hours && hours < 48 {
		when = "Tomorrow, " + date.Format("January 2")
	} else {
		when = date.Format("Monday, January 2")
	}

	switch day.FastLevel {
	case 0:
		text = when + ", there is no fast."
	case 1:
		// normal weekly fast
		if len(day.FastException) > 0 {
			text = fmt.Sprintf("%s, there is a fast: %s.", when, day.FastException)
		} else {
			text = fmt.Sprintf("%s, there is a fast.", when)
		}
	default:
		// One of the four great fasts
		if len(day.FastException) > 0 {
			text = fmt.Sprintf("%s, is during the %s: %s.", when, day.FastLevelDesc, day.FastException)
		} else {
			text = fmt.Sprintf("%s, is during the %s.", when, day.FastLevelDesc)
		}
	}

	return text
}
