package main

import (
	"context"
	"fmt"
	"github.com/brianglass/orthocal"
	"io"
	"strings"
	"time"
	"unicode"
)

const (
	CalendarWrapWidth = 60
	CalendarName      = "Orthodox Feasts and Fasts"
	CalendarTTL       = 12 // hours
	WebBaseURL        = "https://orthocal.info"
)

func GenerateCalendar(ctx context.Context, writer io.Writer, start time.Time, numDays int, factory *orthocal.DayFactory, title string) {
	today := time.Now().In(TZ)

	fmt.Fprintf(writer, "BEGIN:VCALENDAR\r\n")
	fmt.Fprintf(writer, "PRODID:-//brianglass//Orthocal//en\r\n")
	fmt.Fprintf(writer, "VERSION:2.0\r\n")
	fmt.Fprintf(writer, "NAME:%s (%s)\r\n", CalendarName, title)
	fmt.Fprintf(writer, "X-WR-CALNAME:%s (%s)\r\n", CalendarName, title)
	fmt.Fprintf(writer, "REFRESH-INTERVAL;VALUE=DURATION:PT%dH\r\n", CalendarTTL)
	fmt.Fprintf(writer, "X-PUBLISHED-TTL:PT%dH\r\n", CalendarTTL)
	fmt.Fprintf(writer, "TIMEZONE-ID:%s\r\n", TimeZone)
	fmt.Fprintf(writer, "X-WR-TIMEZONE:%s\r\n", TimeZone)

	for i := 0; i < numDays; i++ {
		date := start.AddDate(0, 0, i)
		day := factory.NewDayWithContext(ctx, date.Year(), int(date.Month()), date.Day(), nil)
		uid := fmt.Sprintf("%s.%s@orthocal.info", date.Format("2006-01-02"), title)

		fmt.Fprintf(writer, "BEGIN:VEVENT\r\n")
		fmt.Fprintf(writer, "UID:%s\r\n", uid)
		fmt.Fprintf(writer, "DTSTAMP:%s\r\n", today.Format("20060102T150405Z"))
		fmt.Fprintf(writer, "DTSTART:%s\r\n", date.Format("20060102"))
		fmt.Fprintf(writer, "SUMMARY:%s\r\n", strings.Join(day.Titles, "; "))
		fmt.Fprintf(writer, "DESCRIPTION:%s\r\n", icalDescription(day))
		fmt.Fprintf(writer, "URL:%s/calendar/%s/%d/%d/%d\r\n", WebBaseURL, strings.ToLower(title), date.Year(), int(date.Month()), date.Day())
		fmt.Fprintf(writer, "CLASS:PUBLIC\r\n")
		fmt.Fprintf(writer, "END:VEVENT\r\n")
	}

	fmt.Fprintf(writer, "END:VCALENDAR")
}

func icalDescription(day *orthocal.Day) string {
	var s string

	feasts := strings.Join(day.Feasts, "; ")
	if len(feasts) > 0 {
		s += feasts + `\n\n`
	}

	saints := strings.Join(day.Saints, "; ")
	if len(saints) > 0 {
		s += saints + `\n\n`
	}

	if len(day.FastExceptionDesc) > 0 && day.FastLevel > 0 {
		s += fmt.Sprintf("%s \u2013 %s\\n\\n", day.FastLevelDesc, day.FastExceptionDesc)
	} else {
		s += fmt.Sprintf("%s\\n\\n", day.FastLevelDesc)
	}

	for _, r := range day.Readings {
		if len(r.Description) > 0 {
			s += fmt.Sprintf(`%s (%s, %s)\n`, r.Display, r.Source, r.Description)
		} else {
			s += fmt.Sprintf(`%s (%s)\n`, r.Display, r.Source)
		}
	}

	s = strings.Replace(s, ";", `\;`, -1)
	s = strings.Replace(s, ",", `\,`, -1)

	return icalWrap(s)
}

func icalWrap(text string) string {
	if len(text) <= CalendarWrapWidth {
		return text
	}

	for i := CalendarWrapWidth; i > 0; i-- {
		if unicode.IsSpace(rune(text[i])) {
			return text[:i] + "\r\n " + icalWrap(text[i:])
		}
	}

	return text[:CalendarWrapWidth] + "\r\n " + icalWrap(text[CalendarWrapWidth:])
}
