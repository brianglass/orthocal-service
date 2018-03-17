package main

import (
	"database/sql"
	"fmt"
	"github.com/brianglass/orthocal"
	"github.com/gorilla/mux"
	alexa "github.com/mikeflynn/go-alexa/skillserver"
	"io/ioutil"
	"log"
	"regexp"
	"strings"
	"time"
)

var (
	markupRe = regexp.MustCompile(`<.*?>`)
	refRe    = regexp.MustCompile(`(\d*)\s*([\w\s]+)\s+(\d+)`)
)

var epistles = map[string]string{
	"acts":          "The Acts of the Apostles",
	"romans":        "Saint Paul's letter to the Romans",
	"corinthians":   "Saint Paul's <say-as interpret-as=\"ordinal\">%s</say-as> letter to the Corinthians",
	"galatians":     "Saint Paul's letter to the Galatians",
	"ephesians":     "Saint Paul's letter to the Ephesianss",
	"philipians":    "Saint Paul's letter to the Philipians",
	"colosians":     "Saint Paul's letter to the Colosians",
	"thessalonians": "Saint Paul's <say-as interpret-as=\"ordinal\">%s</say-as> letter to the Thessalonians",
	"timothy":       "Saint Paul's <say-as interpret-as=\"ordinal\">%s</say-as> letter to Timothy",
	"titus":         "Saint Paul's letter to Titus",
	"philemon":      "Saint Paul's letter to Philemon",
	"hebrews":       "Saint Paul's letter to the Hebrews",
	"james":         "The Catholic letter of Saint James",
	"john":          "The <say-as interpret-as=\"ordinal\">%s</say-as> Catholic letter of Saint John",
	"jude":          "The Catholic letter of Saint Jude",
}

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
	day := factory.NewDay(today.Year(), int(today.Month()), today.Day(), nil)

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
	builder.AppendParagraph("Would you like me to read the scriptures?")
	speach := builder.Build()

	// Prepare to read the first reading
	response.EndSession(false)
	response.SessionAttributes["original_intent"] = "Launch"
	response.SessionAttributes["next_reading"] = 0

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
	case "Fasting":
		day := factory.NewDay(date.Year(), int(date.Month()), date.Day(), nil)
		text := FastingSpeach(day)
		response.OutputSpeech(text).Card("Fasting", text)
	case "Scriptures":
		day := factory.NewDay(date.Year(), int(date.Month()), date.Day(), self.bible)

		// Card display
		card := fmt.Sprintf("Readings for %s:\n\n", date.Format("Monday, January 2"))
		for _, reading := range day.Readings {
			card += reading.Display + "\n"
		}

		// build the Speach; we read the first reading on the initial intent
		// request and subsequent readings on following AMAZON.YesIntent
		// requests.
		builder := alexa.NewSSMLTextBuilder()
		builder.AppendParagraph(fmt.Sprintf("There are %d readings for %s.", len(day.Readings), date.Format("Monday, January 2")))
		builder.AppendBreak("strong", "1500ms")
		ReadingSpeach(builder, day.Readings[0])
		builder.AppendBreak("medium", "500ms")

		// Prepare to read the second reading
		response.SessionAttributes["original_intent"] = "Scriptures"
		if len(day.Readings) > 1 {
			response.EndSession(false)
			response.SessionAttributes["next_reading"] = 1
			builder.AppendParagraph("Would you like to hear the next reading?")
		} else {
			response.EndSession(true)
			builder.AppendParagraph("That is the end of the readings.")
		}

		speach := builder.Build()
		response.OutputSpeechSSML(speach).Card("Daily Readings", card)
	case "AMAZON.YesIntent":
		if intent, ok := request.Session.Attributes["original_intent"]; ok {
			switch intent {
			case "Launch", "Scriptures":
				// Here we read a single one of the day's readings, tracking
				// where we are in the Alexa session.
				var nextReading int

				day := factory.NewDay(date.Year(), int(date.Month()), date.Day(), self.bible)

				// Grab the next reading from the session
				if next_reading, ok := request.Session.Attributes["next_reading"]; ok {
					nextReading = int(next_reading.(float64))
				}

				if nextReading < len(day.Readings) {
					reading := day.Readings[nextReading]

					builder := alexa.NewSSMLTextBuilder()
					ReadingSpeach(builder, reading)
					builder.AppendBreak("medium", "500ms")

					// Prepare to read the next reading (or stop if we run out)
					response.SessionAttributes["original_intent"] = intent
					if nextReading+1 >= len(day.Readings) {
						response.EndSession(true)
						response.SessionAttributes["next_reading"] = nil
						builder.AppendParagraph("That is the end of the readings.")
					} else {
						response.EndSession(false)
						response.SessionAttributes["next_reading"] = nextReading + 1
						builder.AppendParagraph("Would you like to hear the next reading?")
					}

					speach := builder.Build()
					response.OutputSpeechSSML(speach)
				}
			default:
			}
		}
	case "AMAZON.NoIntent":
		response.EndSession(true)
	case "AMAZON.HelpIntent":
		content, e := ioutil.ReadFile("templates/help.ssml")
		if e != nil {
			log.Println(e.Error())
			return
		}

		speach := string(content)
		card := markupRe.ReplaceAllString(speach, "")

		response.OutputSpeechSSML(speach).Card("Daily Readings", card)
	case "AMAZON.StopIntent":
	case "AMAZON.CancelIntent":
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

func ReadingSpeach(builder *alexa.SSMLTextBuilder, reading orthocal.Reading) {
	reference := ReferenceSpeach(reading)

	builder.AppendParagraph("The reading is from " + reference + ".")
	builder.AppendBreak("medium", "750ms")

	if len(reading.Passage) == 0 {
		builder.AppendParagraph("Orthodox Daily could not find that reading.")
	}

	for _, verse := range reading.Passage {
		text := markupRe.ReplaceAllString(verse.Content, "")
		builder.AppendParagraph(text)
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
		format, _ := epistles[strings.ToLower(book)]
		if len(number) > 0 {
			format, _ := epistles[strings.ToLower(book)]
			name := fmt.Sprintf(format, number)
			speach = fmt.Sprintf("%s, chapter %s", name, chapter)
		} else {
			speach = fmt.Sprintf("%s, chapter %s", format, chapter)
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
