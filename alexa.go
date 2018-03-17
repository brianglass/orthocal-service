package main

import (
	"database/sql"
	"fmt"
	"github.com/brianglass/orthocal"
	"github.com/gorilla/mux"
	alexa "github.com/mikeflynn/go-alexa/skillserver"
	"log"
	"regexp"
	"strings"
	"time"
)

var (
	markupRe = regexp.MustCompile(`<.*?>`)
	refRe    = regexp.MustCompile(`(\d*)\s*([\w\s]+)\s+(\d+)`)
)

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
	today := time.Now().In(TZ)
	factory := orthocal.NewDayFactory(self.useJulian, self.doJump, self.db)
	day := factory.NewDay(today.Year(), int(today.Month()), today.Day(), self.bible)

	// Create the Card text
	card := "Today is the " + day.Titles[0] + ".\n\n"
	if len(day.FastException) > 0 {
		card += fmt.Sprintf("%s \u2013 %s\n\n", day.FastLevelDesc, day.FastException)
	} else {
		card += fmt.Sprintf("%s\n\n", day.FastLevelDesc)
	}
	for _, reading := range day.Readings {
		card += reading.Display + "\n"
	}

	// Create the speach
	builder := alexa.NewSSMLTextBuilder()

	builder.AppendParagraph("Today is the " + day.Titles[0] + ".")
	builder.AppendParagraph(FastingSpeach(day))
	ScripturesSpeach(builder, day, today)

	speach := builder.Build()
	response.OutputSpeechSSML(speach).Card("About Today", card)
}

func (self *Skill) intentHandler(request *alexa.EchoRequest, response *alexa.EchoResponse) {
	var date time.Time

	factory := orthocal.NewDayFactory(self.useJulian, self.doJump, self.db)

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

	switch request.GetIntentName() {
	case "FastingIntent":
		day := factory.NewDay(date.Year(), int(date.Month()), date.Day(), nil)
		text := FastingSpeach(day)
		response.OutputSpeech(text).Card("Fasting", text)
	case "ReadIntent":
		day := factory.NewDay(date.Year(), int(date.Month()), date.Day(), self.bible)

		card := fmt.Sprintf("Readings for %s:\n\n", date.Format("Monday, January 2"))
		for _, reading := range day.Readings {
			card += reading.Display + "\n"
		}

		builder := alexa.NewSSMLTextBuilder()
		ScripturesSpeach(builder, day, date)
		speach := builder.Build()

		response.OutputSpeechSSML(speach).Card("Daily Readings", card)
	}
}

func FastingSpeach(day *orthocal.Day) string {
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

func ScripturesSpeach(builder *alexa.SSMLTextBuilder, day *orthocal.Day, date time.Time) {
	now := time.Now().In(TZ)

	if day.Month == int(now.Month()) && day.Day == now.Day() && day.Year == now.Year() {
		builder.AppendParagraph(fmt.Sprintf("There are %d readings today.", len(day.Readings)))
	} else {
		builder.AppendParagraph(fmt.Sprintf("There are %d readings for %s.", len(day.Readings), date.Format("Monday, January 2")))
	}

	for _, reading := range day.Readings {
		reference := ReferenceSpeach(reading)

		builder.AppendBreak("strong", "1500ms")
		builder.AppendParagraph("The reading is from " + reference + ".")
		builder.AppendParagraph("Let us attend.")
		builder.AppendBreak("medium", "750ms")

		for _, verse := range reading.Passage {
			text := markupRe.ReplaceAllString(verse.Content, "")
			builder.AppendParagraph(text)
		}
	}
}

func ReferenceSpeach(reading orthocal.Reading) string {
	var speach string

	groups := refRe.FindStringSubmatch(reading.Display)
	number, book, chapter := groups[1], groups[2], groups[3]

	switch reading.Book {
	case "Matthew":
		fallthrough
	case "Mark":
		fallthrough
	case "Luke":
		fallthrough
	case "John":
		speach = fmt.Sprintf("The Gospel according to %s, chapter %s", book, chapter)
	case "Apostol":
		if len(number) > 0 {
			speach = fmt.Sprintf("The <say-as interpret-as=\"ordinal\">%s</say-as> epistle to the %s, chapter %s", number, book, chapter)
		} else {
			speach = fmt.Sprintf("The epistle to the %s, chapter %s", book, chapter)
		}
	case "OT":
		if len(number) > 0 {
			speach = fmt.Sprintf("<say-as interpret-as=\"ordinal\">%s</say-as> %s, chapter %s", number, book, chapter)
		} else {
			speach = fmt.Sprintf("%s, chapter %s", book, chapter)
		}
	default:
		speach = strings.Replace(reading.Display, ".", ":", -1)
	}

	return speach
}
