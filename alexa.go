package main

import (
	"database/sql"
	"fmt"
	alexa "github.com/brianglass/go-alexa/skillserver"
	"github.com/brianglass/orthocal"
	"io/ioutil"
	"log"
	"time"
)

type Skill struct {
	db        *sql.DB
	bible     orthocal.Bible
	useJulian bool
	doJump    bool
	tz        *time.Location
}

func NewSkill(appid string, db *sql.DB, useJulian, doJump bool, bible orthocal.Bible, tz *time.Location) alexa.EchoApplication {
	var skill Skill

	skill.db = db
	skill.bible = bible
	skill.useJulian = useJulian
	skill.doJump = doJump
	skill.tz = tz

	return alexa.EchoApplication{
		AppID:    appid,
		OnLaunch: skill.launchHandler,
		OnIntent: skill.intentHandler,
	}
}

func (self *Skill) launchHandler(request *alexa.EchoRequest, response *alexa.EchoResponse) {
	now := time.Now().In(self.tz)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, self.tz)
	factory := orthocal.NewDayFactory(self.useJulian, self.doJump, self.db)
	day := factory.NewDay(today.Year(), int(today.Month()), today.Day(), nil)

	// Create the speech
	builder := alexa.NewSSMLTextBuilder()
	card := DaySpeech(builder, day, self.tz)
	builder.AppendParagraph(fmt.Sprintf("There are %d scripture readings.", len(day.Readings)))
	builder.AppendParagraph("Would you like to hear the readings?")
	speech := builder.Build()

	// Prepare to read the first reading
	response.EndSession(false)
	response.SessionAttributes["original_intent"] = "Launch"
	response.SessionAttributes["next_reading"] = 0
	response.SessionAttributes["date"] = today.Format("2006-01-02")

	response.OutputSpeechSSML(speech).Card("About Today", card)
}

func (self *Skill) intentHandler(request *alexa.EchoRequest, response *alexa.EchoResponse) {
	var date time.Time

	factory := orthocal.NewDayFactory(self.useJulian, self.doJump, self.db)

	if when, e := request.GetSlotValue("date"); e == nil && len(when) > 0 {
		date, e = time.ParseInLocation("2006-01-02", when, self.tz)
		if e != nil {
			response.OutputSpeech("I didn't understand the date you requested.")
			return
		}
	} else {
		now := time.Now().In(self.tz)
		date = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, self.tz)
	}

	switch request.GetIntentName() {
	case "Day":
		day := factory.NewDay(date.Year(), int(date.Month()), date.Day(), nil)
		builder := alexa.NewSSMLTextBuilder()
		card := DaySpeech(builder, day, self.tz)
		when := WhenSpeach(day, self.tz)
		speech := builder.Build()
		response.OutputSpeechSSML(speech).Card("About "+when, card)
	case "Scriptures":
		// build the scriptures Speech; we read the first reading on the
		// initial Scriptures intent request and subsequent readings on
		// following AMAZON.YesIntent (or AMAZON.NextIntent) requests.
		var nextReading int

		day := factory.NewDay(date.Year(), int(date.Month()), date.Day(), self.bible)

		// Card display
		card := fmt.Sprintf("Readings for %s:\n\n", date.Format("Monday, January 2"))
		for _, reading := range day.Readings {
			card += reading.Display + "\n"
		}

		reading := day.Readings[nextReading]
		groupSize := EstimateGroupSize(reading.Passage)

		// It's a bit expensive to rebuild the speech if the response is too
		// long, but it should be a relatively rare exception and is the most
		// accurate way to make the decision.
		builder := alexa.NewSSMLTextBuilder()
		builder.AppendParagraph(fmt.Sprintf("There are %d readings for %s.", len(day.Readings), date.Format("Monday, January 2")))
		builder.AppendBreak("strong", "1500ms")
		if groupSize > 0 {
			ReadingSpeech(builder, reading, groupSize)
		} else {
			ReadingSpeech(builder, reading, -1)
		}
		builder.AppendBreak("medium", "750ms")

		// Prepare to read the second reading or verse group
		response.SessionAttributes["original_intent"] = "Scriptures"
		if groupSize > 0 {
			// We need to break the passage up into groups of verses
			response.EndSession(false)
			response.SessionAttributes["next_reading"] = nextReading
			response.SessionAttributes["next_verse"] = groupSize
			response.SessionAttributes["group_size"] = groupSize
			response.SessionAttributes["date"] = date.Format("2006-01-02")
			builder.AppendParagraph("This is a long reading. Would you like me to continue?")
		} else if nextReading+1 < len(day.Readings) {
			// We can move on to the next reading
			response.EndSession(false)
			response.SessionAttributes["next_reading"] = nextReading + 1
			response.SessionAttributes["date"] = date.Format("2006-01-02")
			builder.AppendParagraph("Would you like to hear the next reading?")
		} else {
			// There are no more readings, so we end the session
			response.EndSession(true)
			builder.AppendParagraph("That is the end of the readings.")
		}

		speech := builder.Build()
		response.OutputSpeechSSML(speech).Card("Daily Readings", card)

	case "AMAZON.YesIntent", "AMAZON.NextIntent":
		if intent, ok := request.Session.Attributes["original_intent"]; ok {
			switch intent {
			case "Launch", "Scriptures":
				// Here we read a single one of the day's readings, tracking
				// where we are in the Alexa session.
				var nextReading, groupSize, nextVerse int

				// Get the date from the session; barf if there isn't one
				if dateString, ok := request.Session.Attributes["date"]; ok {
					var e error
					date, e = time.ParseInLocation("2006-01-02", dateString.(string), self.tz)
					if e != nil {
						response.OutputSpeech("I didn't understand the date you requested.")
						return
					}
				} else {
					response.EndSession(true)
					response.OutputSpeech("I'm not sure what you mean in this context.")
					return
				}

				day := factory.NewDay(date.Year(), int(date.Month()), date.Day(), self.bible)

				// Grab the next reading from the session
				if next_reading, ok := request.Session.Attributes["next_reading"]; ok {
					nextReading = int(next_reading.(float64))
					if nextReading >= len(day.Readings) {
						// This should never happen
						response.EndSession(true)
						response.OutputSpeech("There are no more readings.")
						return
					}
				} else {
					// This should never happen
					response.EndSession(true)
					response.OutputSpeech("I don't know what you mean in this context.")
					return
				}

				reading := day.Readings[nextReading]

				// If we have to break up long passages, we need to get the next verse
				if group_size, ok := request.Session.Attributes["group_size"]; ok {
					groupSize = int(group_size.(float64))
					if next_verse, ok := request.Session.Attributes["next_verse"]; ok {
						nextVerse = int(next_verse.(float64))
					}
				} else {
					groupSize = EstimateGroupSize(reading.Passage)
				}

				builder := alexa.NewSSMLTextBuilder()
				if nextVerse > 0 {
					ReadingRangeSpeech(builder, reading, nextVerse, nextVerse+groupSize)
				} else if groupSize > 0 {
					ReadingSpeech(builder, reading, groupSize)
				} else {
					ReadingSpeech(builder, reading, -1)
				}
				builder.AppendBreak("medium", "750ms")

				// Prepare to read the next reading or verse group (or stop if we run out)
				response.SessionAttributes["original_intent"] = intent
				response.SessionAttributes["date"] = date.Format("2006-01-02")
				if groupSize > 0 && nextVerse+groupSize < len(reading.Passage) {
					// We need to break the passage up into groups of verses
					response.EndSession(false)
					response.SessionAttributes["next_reading"] = nextReading
					response.SessionAttributes["next_verse"] = nextVerse + groupSize
					response.SessionAttributes["group_size"] = groupSize
					builder.AppendParagraph("This is a long reading. Would you like me to continue?")
				} else if nextReading+1 >= len(day.Readings) {
					// There are no more readings, so we end the session
					response.EndSession(true)
					builder.AppendParagraph("That is the end of the readings.")
				} else {
					// We can move on to the next reading
					response.EndSession(false)
					response.SessionAttributes["next_reading"] = nextReading + 1
					delete(response.SessionAttributes, "next_verse")
					delete(response.SessionAttributes, "group_size")
					builder.AppendParagraph("Would you like to hear the next reading?")
				}

				speech := builder.Build()
				response.OutputSpeechSSML(speech)
			default:
				response.EndSession(true)
				response.OutputSpeech("I'm not sure what you mean in this context.")
				return
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

		speech := string(content)
		card := markupRe.ReplaceAllString(speech, "")

		// Clear out session
		response.EndSession(false)
		delete(response.SessionAttributes, "date")
		delete(response.SessionAttributes, "next_reading")
		delete(response.SessionAttributes, "original_intent")

		response.OutputSpeechSSML(speech).Card("Help", card)
	case "AMAZON.StopIntent":
	case "AMAZON.CancelIntent":
	}
}
