package main

import (
	"fmt"
	"github.com/brianglass/orthocal"
	"io"
	"strings"
	"time"
	"unicode"
)

func GenerateCalendar(writer io.Writer, start time.Time, numDays int, factory *orthocal.DayFactory) {
	today := time.Now().In(TZ)

	fmt.Fprintf(writer, "BEGIN:VCALENDAR\r\n")
	fmt.Fprintf(writer, "PRODID:-//brianglass//Orthocal//en\r\n")
	fmt.Fprintf(writer, "VERSION:2.0\r\n")
	fmt.Fprintf(writer, "NAME:%s\r\n", CalendarName)
	fmt.Fprintf(writer, "X-WR-CALNAME:%s\r\n", CalendarName)
	fmt.Fprintf(writer, "REFRESH-INTERVAL;VALUE=DURATION:PT%dH\r\n", CalendarTTL)
	fmt.Fprintf(writer, "X-PUBLISHED-TTL:PT%dH\r\n", CalendarTTL)
	fmt.Fprintf(writer, "TIMEZONE-ID:%s\r\n", CalendarTZ)
	fmt.Fprintf(writer, "X-WR-TIMEZONE:%s\r\n", CalendarTZ)

	for i := 0; i < numDays; i++ {
		date := start.AddDate(0, 0, i)
		day := factory.NewDay(date.Year(), int(date.Month()), date.Day(), nil)
		uid := date.Format("2006-01-02") + "@orthocal.info"

		fmt.Fprintf(writer, "BEGIN:VEVENT\r\n")
		fmt.Fprintf(writer, "UID:%s\r\n", uid)
		fmt.Fprintf(writer, "DTSTAMP:%s\r\n", today.Format("20060102T150405Z"))
		fmt.Fprintf(writer, "DTSTART:%s\r\n", date.Format("20060102"))
		fmt.Fprintf(writer, "SUMMARY:%s\r\n", strings.Join(day.Titles, "; "))
		fmt.Fprintf(writer, "DESCRIPTION:%s\r\n", icalDescription(day))
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

	if len(day.FastException) > 0 {
		s += fmt.Sprintf("%s \u2013 %s\\n\\n", day.FastLevel, day.FastException)
	} else {
		s += fmt.Sprintf("%s\\n\\n", day.FastLevel)
	}

	for _, r := range day.Readings {
		s += fmt.Sprintf(`%s\n`, r.Display)
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
