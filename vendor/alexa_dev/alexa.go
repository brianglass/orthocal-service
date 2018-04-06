package alexa_dev

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

const (
	maxSpeechLength = 8000
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
	"ephesians":     "Saint Paul's letter to the Ephesians",
	"philippians":   "Saint Paul's letter to the Philippians",
	"colossians":    "Saint Paul's letter to the Colossians",
	"thessalonians": "Saint Paul's <say-as interpret-as=\"ordinal\">%s</say-as> letter to the Thessalonians",
	"timothy":       "Saint Paul's <say-as interpret-as=\"ordinal\">%s</say-as> letter to Timothy",
	"titus":         "Saint Paul's letter to Titus",
	"philemon":      "Saint Paul's letter to Philemon",
	"hebrews":       "Saint Paul's letter to the Hebrews",
	"james":         "The Catholic letter of Saint James",
	"peter":         "The <say-as interpret-as=\"ordinal\">%s</say-as> Catholic letter of Saint Peter",
	"john":          "The <say-as interpret-as=\"ordinal\">%s</say-as> Catholic letter of Saint John",
	"jude":          "The Catholic letter of Saint Jude",
}

type Skill struct {
	db        *sql.DB
	bible     *orthocal.Bible
	useJulian bool
	doJump    bool
	tz        *time.Location
	appid     string
}

func NewSkill(router *mux.Router, appid string, db *sql.DB, useJulian, doJump bool, bible *orthocal.Bible, tz *time.Location) *Skill {
	var skill Skill

	skill.appid = appid
	skill.db = db
	skill.useJulian = useJulian
	skill.doJump = doJump
	skill.bible = bible
	skill.tz = tz

	/*
		apps := map[string]interface{}{
			"/dev/echo/": alexa.EchoApplication{
				AppID:    appid,
				OnLaunch: skill.launchHandler,
				OnIntent: skill.intentHandler,
			},
		}

		alexa.Init(apps, router)
	*/

	return &skill
}

func (self *Skill) GetEchoApplication() alexa.EchoApplication {
	return alexa.EchoApplication{
		AppID:    self.appid,
		OnLaunch: self.launchHandler,
		OnIntent: self.intentHandler,
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

		day := factory.NewDay(date.Year(), int(date.Month()), date.Day(), self.bible)

		// Card display
		card := fmt.Sprintf("Readings for %s:\n\n", date.Format("Monday, January 2"))
		for _, reading := range day.Readings {
			card += reading.Display + "\n"
		}

		// It's a bit expensive to rebuild the speech if the response is too
		// long, but it should be a relatively rare exception and is the most
		// accurate way to make the decision.
		for {
			var groupSize int

			builder := alexa.NewSSMLTextBuilder()
			builder.AppendParagraph(fmt.Sprintf("There are %d readings for %s.", len(day.Readings), date.Format("Monday, January 2")))
			builder.AppendBreak("strong", "1500ms")
			if groupSize > 0 {
				ReadingSpeech(builder, day.Readings[0], groupSize)
			} else {
				ReadingSpeech(builder, day.Readings[0], -1)
			}
			builder.AppendBreak("medium", "750ms")

			// Prepare to read the second reading
			response.SessionAttributes["original_intent"] = "Scriptures"
			if groupSize > 0 {
				response.EndSession(false)
				response.SessionAttributes["next_reading"] = 0
				response.SessionAttributes["next_verse"] = groupSize
				response.SessionAttributes["group_size"] = groupSize
				response.SessionAttributes["date"] = date.Format("2006-01-02")
				builder.AppendParagraph("This is a long reading. Would you like me to continue?")
			} else if len(day.Readings) > 1 {
				response.EndSession(false)
				response.SessionAttributes["next_reading"] = 1
				response.SessionAttributes["date"] = date.Format("2006-01-02")
				builder.AppendParagraph("Would you like to hear the next reading?")
			} else {
				response.EndSession(true)
				builder.AppendParagraph("That is the end of the readings.")
			}

			speech := builder.Build()

			if len(speech) <= maxSpeechLength {
				response.OutputSpeechSSML(speech).Card("Daily Readings", card)
				return
			} else {
				// This is definitely not exact, but should be a pretty good approximation
				groupCount := len(speech)/maxSpeechLength + 1
				groupSize = len(speech) / groupCount
				if len(speech)%groupCount > 0 {
					groupSize++
				}
			}
		}

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
				}

				// If we have to break up long passages, we need to get the next verse
				if group_size, ok := request.Session.Attributes["group_size"]; ok {
					groupSize = int(group_size.(float64))
				}

				// If we have to break up long passages, we need to get the next verse
				if next_verse, ok := request.Session.Attributes["next_verse"]; ok {
					nextVerse = int(next_verse.(float64))
				}

				if nextReading >= len(day.Readings) {
					// This should never happen
					response.EndSession(true)
					response.OutputSpeech("There are no more readings.")
					return
				}

				reading := day.Readings[nextReading]

				for {
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
						response.EndSession(false)
						response.SessionAttributes["next_reading"] = nextReading
						response.SessionAttributes["next_verse"] = nextVerse + groupSize
						response.SessionAttributes["group_size"] = groupSize
						builder.AppendParagraph("This is a long reading. Would you like me to continue?")
					} else if nextReading+1 >= len(day.Readings) {
						response.EndSession(true)
						response.SessionAttributes["next_reading"] = nil
						builder.AppendParagraph("That is the end of the readings.")
					} else {
						response.EndSession(false)
						response.SessionAttributes["next_reading"] = nextReading + 1
						delete(response.SessionAttributes, "next_verse")
						delete(response.SessionAttributes, "group_size")
						builder.AppendParagraph("Would you like to hear the next reading?")
					}

					speech := builder.Build()

					if len(speech) <= maxSpeechLength {
						response.OutputSpeechSSML(speech)
						return
					} else {
						// This is definitely not exact, but should be a pretty good approximation
						groupCount := len(speech)/maxSpeechLength + 1
						groupSize = len(speech) / groupCount
						if len(speech)%groupCount > 0 {
							groupSize++
						}
					}
				}
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

func DaySpeech(builder *alexa.SSMLTextBuilder, day *orthocal.Day, tz *time.Location) string {
	var feasts, saints string

	when := WhenSpeach(day, tz)

	// Commemorations
	if len(day.Feasts) > 1 {
		feasts = fmt.Sprintf("The feasts celebrated are: %s.", HumanJoin(day.Feasts))
	} else if len(day.Feasts) == 1 {
		feasts = fmt.Sprintf("The feast of %s is celebrated.", day.Feasts[0])
	}
	if len(day.Saints) > 1 {
		saints = fmt.Sprintf("The commemorations are for %s.", HumanJoin(day.Saints))
	} else if len(day.Saints) == 1 {
		saints = fmt.Sprintf("The commemoration is for %s.", day.Saints[0])
	}

	// Create the Card text
	card := when + ", is the " + day.Titles[0] + ".\n\n"
	if len(day.FastException) > 0 {
		card += fmt.Sprintf("%s \u2013 %s\n\n", day.FastLevelDesc, day.FastException)
	} else {
		card += fmt.Sprintf("%s\n\n", day.FastLevelDesc)
	}
	if len(feasts) > 0 {
		card += feasts + "\n\n"
	}
	if len(saints) > 0 {
		card += saints + "\n\n"
	}
	for _, reading := range day.Readings {
		card += reading.Display + "\n"
	}

	// Create the speech
	builder.AppendParagraph(when + ", is the " + day.Titles[0] + ".")
	builder.AppendParagraph(FastingSpeech(day))
	builder.AppendParagraph(feasts)
	builder.AppendParagraph(strings.Replace(saints, "Ven.", `<sub alias="The Venerable">Ven.</sub>`, -1))

	return card
}

func WhenSpeach(day *orthocal.Day, tz *time.Location) string {
	var when string

	now := time.Now().In(tz)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, tz)
	date := time.Date(day.Year, time.Month(day.Month), day.Day, 0, 0, 0, 0, tz)

	hours := date.Sub(today).Hours()
	if 0 <= hours && hours < 24 {
		when = "Today, " + date.Format("January 2")
	} else if 24 <= hours && hours < 48 {
		when = "Tomorrow, " + date.Format("January 2")
	} else {
		when = date.Format("Monday, January 2")
	}

	return when
}

func FastingSpeech(day *orthocal.Day) string {
	var text string

	switch day.FastLevel {
	case 0:
		text = "On this day there is no fast."
	case 1:
		// normal weekly fast
		if len(day.FastException) > 0 {
			text = fmt.Sprintf("On this day there is a fast. %s.", day.FastException)
		} else {
			text = fmt.Sprintf("On this day there is a fast.")
		}
	default:
		// One of the four great fasts
		if len(day.FastException) > 0 {
			text = fmt.Sprintf("This day is during the %s. %s.", day.FastLevelDesc, day.FastException)
		} else {
			text = fmt.Sprintf("This day is during the %s.", day.FastLevelDesc)
		}
	}

	return text
}

func ReadingSpeech(builder *alexa.SSMLTextBuilder, reading orthocal.Reading, end int) {
	reference := ReferenceSpeech(reading)

	builder.AppendParagraph("The reading is from " + reference + ".")
	builder.AppendBreak("medium", "750ms")

	if len(reading.Passage) == 0 {
		builder.AppendParagraph("Orthodox Daily could not find that reading.")
		return
	}

	if end > 0 {
		for i := 0; i < end; i++ {
			text := markupRe.ReplaceAllString(reading.Passage[i].Content, "")
			builder.AppendParagraph(text)
		}
	} else {
		for _, verse := range reading.Passage {
			text := markupRe.ReplaceAllString(verse.Content, "")
			builder.AppendParagraph(text)
		}
	}
}

func ReadingRangeSpeech(builder *alexa.SSMLTextBuilder, reading orthocal.Reading, start, end int) {
	for i := start; i < end; i++ {
		text := markupRe.ReplaceAllString(reading.Passage[i].Content, "")
		builder.AppendParagraph(text)
	}
}

func ReferenceSpeech(reading orthocal.Reading) string {
	var speech string

	groups := refRe.FindStringSubmatch(reading.Display)
	if len(groups) < 4 {
		// The reference is irregular so we just let Alexa do the best she can
		return strings.Replace(reading.Display, ".", ":", -1)
	}

	// The book here is the book of the Bible whereas the book below is the
	// liturgical book
	number, book, chapter := groups[1], groups[2], groups[3]

	switch strings.ToLower(reading.Book) {
	case "matthew", "mark", "luke", "john":
		speech = fmt.Sprintf("The Holy Gospel according to Saint %s, chapter %s", book, chapter)
	case "apostol":
		format, ok := epistles[strings.ToLower(book)]
		if !ok {
			speech = fmt.Sprintf(book+", chapter %s", chapter)
		} else if len(number) > 0 {
			speech = fmt.Sprintf(format+", chapter %s", number, chapter)
		} else {
			speech = fmt.Sprintf(format+", chapter %s", chapter)
		}
	case "ot":
		if len(number) > 0 {
			speech = fmt.Sprintf("<say-as interpret-as=\"ordinal\">%s</say-as> %s, chapter %s", number, book, chapter)
		} else {
			speech = fmt.Sprintf("%s, chapter %s", book, chapter)
		}
	default:
		speech = strings.Replace(reading.Display, ".", ":", -1)
	}

	return speech
}

func HumanJoin(words []string) string {
	if len(words) > 1 {
		return strings.Join(words[:len(words)-1], ", ") + " and " + words[len(words)-1]
	} else {
		return words[0]
	}
}

func GetReadingLength(reading orthocal.Reading) int {
	var length int

	for _, verse := range reading.Passage {
		text := markupRe.ReplaceAllString(verse.Content, "")
		length += len(text) + len("<p></p>")
	}

	return length
}
