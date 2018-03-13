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

var verseRe = regexp.MustCompile(`</?verse.*?>`)

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
			OnIntent: skill.IntentHandler,
			OnLaunch: skill.IntentHandler,
		},
	}

	alexa.Init(apps, router)

	return &skill
}

func (self *Skill) IntentHandler(request *alexa.EchoRequest, response *alexa.EchoResponse) {
	today := time.Now().In(TZ)

	factory := orthocal.NewDayFactory(self.useJulian, self.doJump, self.db)
	day := factory.NewDay(today.Year(), int(today.Month()), today.Day(), self.bible)

	var passage string
	for _, verse := range day.Readings[0].Passage {
		text := regexp.MustCompile(`<i>`).ReplaceAllString(verse.Content, `<emphasis level="strong">`)
		text = regexp.MustCompile(`</i>`).ReplaceAllString(text, `</emphasis>`)
		passage += verseRe.ReplaceAllString(text, "") + " "
	}

	passage = fmt.Sprintf("<speak>%s</speak>", passage)
	response.OutputSpeechSSML(passage).Card("Orthodox Daily Readings", "This is a test card.")
}
