package main

import (
	"fmt"
	"github.com/brianglass/orthocal"
	alexa "github.com/mikeflynn/go-alexa/skillserver"
	"strings"
	"time"
)

func DaySpeech(builder *alexa.SSMLTextBuilder, day *orthocal.Day, tz *time.Location) (card string) {
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
	card = when + ", is the " + day.Titles[0] + ".\n\n"
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

func WhenSpeach(day *orthocal.Day, tz *time.Location) (when string) {
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

func FastingSpeech(day *orthocal.Day) (text string) {
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
		for i := 0; i < end && i < len(reading.Passage); i++ {
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
	for i := start; i < end && i < len(reading.Passage); i++ {
		text := markupRe.ReplaceAllString(reading.Passage[i].Content, "")
		builder.AppendParagraph(text)
	}
}

func ReferenceSpeech(reading orthocal.Reading) (speech string) {
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

func GetPassageLength(passage orthocal.Passage, start, end int) (length int) {
	if start < 0 {
		start = 0
	}

	if end <= 0 {
		end = len(passage)
	}

	for i := start; i < end && i < len(passage); i++ {
		length += len(passage[i].Content) + len("<p></p>")
	}

	return length
}

func EstimateGroupSize(passage orthocal.Passage) (groupSize int) {
	const (
		prelude       = len(`<p>There are 29 readings for Tuesday, January 3. The reading is from Saint Paul's <say-as interpret-as=\"ordinal\">2</say-as> letter to the Thessalonians</p>`)
		postlude      = len(`<p>Would you like to hear the next reading?</p>`)
		groupPostlude = len(`<p>This is a long reading. Would you like me to continue?</p>`)
	)

	verseCount := len(passage)

	passageLength := prelude + GetPassageLength(passage, 0, 0) + postlude
	if passageLength <= maxSpeechLength {
		// Yay! We don't have to break the passage into groups.
		return -1
	}

	// Start with a good guess and then grow the group count until we find one
	// that fits.
	groupCount := passageLength/maxSpeechLength + 1
	for failed := true; failed; groupCount++ {
		groupSize = verseCount / groupCount
		if verseCount%groupCount > 0 {
			groupSize++
		}

		// loop over the groups and tally up the lengths
		failed = false
		for g := 0; g < groupCount; g++ {
			start := g * groupSize
			end := start + groupSize
			length := GetPassageLength(passage, start, end)

			if g == 0 {
				length += prelude
			}

			if g == groupCount-1 {
				length += postlude
			} else {
				length += groupPostlude
			}

			if length > maxSpeechLength {
				failed = true
				break
			}
		}
	}

	return groupSize
}