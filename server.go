package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/brianglass/orthocal"
	"github.com/gorilla/mux"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const (
	CalendarWrapWidth = 60
	CalendarName      = "Orthodox Feasts and Fasts"
	CalendarTTL       = 12 // hours
	CalendarTZ        = "America/Denver"
	CalendarMaxDays   = 6 * 30
)

var (
	TZ *time.Location
)

type CalendarServer struct {
	db        *sql.DB
	bible     *orthocal.Bible
	useJulian bool
	doJump    bool
}

func init() {
	var e error
	TZ, e = time.LoadLocation(CalendarTZ)
	if e != nil {
		TZ = time.UTC
		log.Printf("Error loading timezone: %s, using UTC.", CalendarTZ)
	}
}

func NewCalendarServer(router *mux.Route, db *sql.DB, useJulian, doJump bool, bible *orthocal.Bible) *CalendarServer {
	var self CalendarServer

	self.db = db
	self.bible = bible
	self.useJulian = useJulian
	self.doJump = doJump

	r := router.Methods("GET").Subrouter()

	r.HandleFunc("/", self.todayHandler)
	r.HandleFunc("/ical/", self.icalHandler)
	r.HandleFunc("/{year}/{month}/", self.monthHandler)
	r.HandleFunc("/{year}/{month}/{day}/", self.dayHandler)

	return &self
}

func (self *CalendarServer) todayHandler(writer http.ResponseWriter, request *http.Request) {
	today := time.Now().In(TZ)
	factory := orthocal.NewDayFactory(self.useJulian, self.doJump, self.db)
	Day := factory.NewDay(today.Year(), int(today.Month()), today.Day(), self.bible)

	writer.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "\t")

	if e := encoder.Encode(Day); e != nil {
		http.Error(writer, "Not Found", http.StatusInternalServerError)
		log.Printf("Could not marshal json for dayHandler: %#n.", e)
	}
}

func (self *CalendarServer) dayHandler(writer http.ResponseWriter, request *http.Request) {
	vars := mux.Vars(request)

	year, e := strconv.Atoi(vars["year"])
	if e != nil {
		http.Error(writer, "Not Found", http.StatusNotFound)
		return
	}

	month, e := strconv.Atoi(vars["month"])
	if e != nil {
		http.Error(writer, "Not Found", http.StatusNotFound)
		return
	}

	day, e := strconv.Atoi(vars["day"])
	if e != nil {
		http.Error(writer, "Not Found", http.StatusNotFound)
		return
	}

	factory := orthocal.NewDayFactory(self.useJulian, self.doJump, self.db)
	Day := factory.NewDay(year, month, day, self.bible)

	writer.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "\t")

	e = encoder.Encode(Day)
	if e != nil {
		http.Error(writer, "Not Found", http.StatusInternalServerError)
		log.Printf("Could not marshal json for dayHandler: %#n.", e)
	}
}

func (self *CalendarServer) monthHandler(writer http.ResponseWriter, request *http.Request) {
	vars := mux.Vars(request)

	year, e := strconv.Atoi(vars["year"])
	if e != nil {
		http.Error(writer, "Not Found", http.StatusNotFound)
		return
	}

	month, e := strconv.Atoi(vars["month"])
	if e != nil {
		http.Error(writer, "Not Found", http.StatusNotFound)
		return
	}

	factory := orthocal.NewDayFactory(self.useJulian, self.doJump, self.db)

	writer.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "\t")

	io.WriteString(writer, "[")
	for day := 1; day <= 31; day++ {
		d := factory.NewDay(year, month, day, nil)

		// NewDay automatically wraps, so break out once we hit the next month
		if d.Month != month {
			break
		}

		if day > 1 {
			io.WriteString(writer, ", ")
		}

		e = encoder.Encode(d)
		if e != nil {
			http.Error(writer, "Server Error", http.StatusInternalServerError)
			log.Printf("Could not marshal json for dayHandler: %#n.", e)
		}
	}
	io.WriteString(writer, "]")
}

func (self *CalendarServer) icalHandler(writer http.ResponseWriter, request *http.Request) {
	today := time.Now().In(TZ)
	factory := orthocal.NewDayFactory(self.useJulian, self.doJump, self.db)

	writer.Header().Set("Content-Type", "text/calendar")

	fmt.Fprintf(writer, "BEGIN:VCALENDAR\r\n")
	fmt.Fprintf(writer, "PRODID:-//brianglass//Orthocal//en\r\n")
	fmt.Fprintf(writer, "VERSION:2.0\r\n")
	fmt.Fprintf(writer, "NAME:%s\r\n", CalendarName)
	fmt.Fprintf(writer, "X-WR-CALNAME:%s\r\n", CalendarName)
	fmt.Fprintf(writer, "REFRESH-INTERVAL;VALUE=DURATION:PT%dH\r\n", CalendarTTL)
	fmt.Fprintf(writer, "X-PUBLISHED-TTL:PT%dH\r\n", CalendarTTL)
	fmt.Fprintf(writer, "TIMEZONE-ID:%s\r\n", CalendarTZ)
	fmt.Fprintf(writer, "X-WR-TIMEZONE:%s\r\n", CalendarTZ)

	for i := 0; i < CalendarMaxDays; i++ {
		date := today.AddDate(0, 0, i)
		day := factory.NewDay(date.Year(), int(date.Month()), date.Day(), nil)
		uid := date.Format("2006-01-02") + "@orthocal.info"

		var title string
		for _, c := range day.Commemorations {
			if len(c.Title) > 0 {
				title = c.Title
				break
			} else if len(c.FeastName) > 0 {
				title = c.FeastName
				break
			}
		}

		fmt.Fprintf(writer, "BEGIN:VEVENT\r\n")
		fmt.Fprintf(writer, "UID:%s\r\n", uid)
		fmt.Fprintf(writer, "DTSTAMP:%s\r\n", today.Format("20060102T150405Z"))
		fmt.Fprintf(writer, "DTSTART:%s\r\n", date.Format("20060102"))
		fmt.Fprintf(writer, "SUMMARY:%s\r\n", title)
		fmt.Fprintf(writer, "DESCRIPTION:%s\r\n", IcalDescription(day))
		fmt.Fprintf(writer, "CLASS:PUBLIC\r\n")
		fmt.Fprintf(writer, "END:VEVENT\r\n")
	}

	fmt.Fprintf(writer, "END:VCALENDAR")
}

func IcalDescription(day *orthocal.Day) string {
	var s string

	for _, c := range day.Commemorations {
		if len(c.Title) > 0 {
			s += fmt.Sprintf(`%s\n`, c.Title)
		} else if len(c.FeastName) > 0 {
			s += fmt.Sprintf(`%s\n`, c.FeastName)
		}
	}

	s += `\n`

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

	return IcalWrap(s)
}

func IcalWrap(text string) string {
	if len(text) <= CalendarWrapWidth {
		return text
	}

	for i := CalendarWrapWidth; i > 0; i-- {
		if unicode.IsSpace(rune(text[i])) {
			return text[:i] + "\r\n " + IcalWrap(text[i:])
		}
	}

	return text[:CalendarWrapWidth] + "\r\n " + IcalWrap(text[CalendarWrapWidth:])
}
