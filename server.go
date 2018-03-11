package main

import (
	"database/sql"
	"encoding/json"
	"github.com/brianglass/orthocal"
	"github.com/gorilla/mux"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"
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
		log.Printf("Error loading '%s' timezone, using UTC.", CalendarTZ)
	}
}

func NewCalendarServer(router *mux.Route, db *sql.DB, useJulian, doJump bool, bible *orthocal.Bible) *CalendarServer {
	var self CalendarServer

	self.db = db
	self.bible = bible
	self.useJulian = useJulian
	self.doJump = doJump

	r := router.Methods("GET", "HEAD").Subrouter()

	r.HandleFunc(`/`, self.todayHandler)
	r.HandleFunc(`/ical/`, self.icalHandler)
	r.HandleFunc(`/{year:\d+}/{month:\d+}/`, self.monthHandler)
	r.HandleFunc(`/{year:\d+}/{month:\d+}/{day:\d+}/`, self.dayHandler)

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

	year, _ := strconv.Atoi(vars["year"])
	month, _ := strconv.Atoi(vars["month"])
	day, _ := strconv.Atoi(vars["day"])

	factory := orthocal.NewDayFactory(self.useJulian, self.doJump, self.db)
	Day := factory.NewDay(year, month, day, self.bible)

	writer.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "\t")

	e := encoder.Encode(Day)
	if e != nil {
		http.Error(writer, "Not Found", http.StatusInternalServerError)
		log.Printf("Could not marshal json for dayHandler: %#n.", e)
	}
}

func (self *CalendarServer) monthHandler(writer http.ResponseWriter, request *http.Request) {
	vars := mux.Vars(request)

	year, _ := strconv.Atoi(vars["year"])
	month, _ := strconv.Atoi(vars["month"])

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

		e := encoder.Encode(d)
		if e != nil {
			http.Error(writer, "Server Error", http.StatusInternalServerError)
			log.Printf("Could not marshal json for dayHandler: %#n.", e)
		}
	}
	io.WriteString(writer, "]")
}

func (self *CalendarServer) icalHandler(writer http.ResponseWriter, request *http.Request) {
	start := time.Now().In(TZ).AddDate(0, 0, -30)
	factory := orthocal.NewDayFactory(self.useJulian, self.doJump, self.db)

	writer.Header().Set("Content-Type", "text/calendar")
	GenerateCalendar(writer, start, CalendarMaxDays, factory)
}
