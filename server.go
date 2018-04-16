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
	CalendarMaxDays = 6 * 30
)

type CalendarServer struct {
	db        *sql.DB
	bible     *orthocal.Bible
	useJulian bool
	doJump    bool
}

func NewCalendarServer(router *mux.Router, db *sql.DB, useJulian, doJump bool, bible *orthocal.Bible) *CalendarServer {
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
	Day := factory.NewDayWithContext(request.Context(), today.Year(), int(today.Month()), today.Day(), self.bible)

	writer.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "\t")

	if e := encoder.Encode(Day); e != nil {
		http.Error(writer, "Internal Server Error", http.StatusInternalServerError)
		log.Printf("Could not marshal json for dayHandler: %#n.", e)
	}
}

func (self *CalendarServer) dayHandler(writer http.ResponseWriter, request *http.Request) {
	vars := mux.Vars(request)

	// Mux is setup to only send things that match this pattern, so we don't
	// need to handle the errors.
	year, _ := strconv.Atoi(vars["year"])
	month, _ := strconv.Atoi(vars["month"])
	day, _ := strconv.Atoi(vars["day"])

	factory := orthocal.NewDayFactory(self.useJulian, self.doJump, self.db)
	Day := factory.NewDayWithContext(request.Context(), year, month, day, self.bible)

	writer.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "\t")

	e := encoder.Encode(Day)
	if e != nil {
		http.Error(writer, "Internal Server Error", http.StatusInternalServerError)
		log.Printf("Could not marshal json for dayHandler: %#n.", e)
	}
}

func (self *CalendarServer) monthHandler(writer http.ResponseWriter, request *http.Request) {
	vars := mux.Vars(request)

	// Mux is setup to only send things that match this pattern, so we don't
	// need to handle the errors.
	year, _ := strconv.Atoi(vars["year"])
	month, _ := strconv.Atoi(vars["month"])

	factory := orthocal.NewDayFactory(self.useJulian, self.doJump, self.db)

	writer.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "\t")

	io.WriteString(writer, "[")
	for day := 1; day <= 31; day++ {
		date := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.Local)

		// NewDay automatically wraps, so break out once we hit the next month
		if date.Month() != time.Month(month) {
			break
		}

		d := factory.NewDayWithContext(request.Context(), date.Year(), int(date.Month()), date.Day(), nil)

		if day > 1 {
			io.WriteString(writer, ", ")
		}

		e := encoder.Encode(d)
		if e != nil {
			http.Error(writer, "Internal Server Error", http.StatusInternalServerError)
			log.Printf("Could not marshal json for dayHandler: %#n.", e)
		}
	}
	io.WriteString(writer, "]")
}

func (self *CalendarServer) icalHandler(writer http.ResponseWriter, request *http.Request) {
	start := time.Now().In(TZ).AddDate(0, 0, -30)
	factory := orthocal.NewDayFactory(self.useJulian, self.doJump, self.db)

	writer.Header().Set("Content-Type", "text/calendar")
	GenerateCalendar(request.Context(), writer, start, CalendarMaxDays, factory)
}
