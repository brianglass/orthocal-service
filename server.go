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
	"time"
)

type CalendarServer struct {
	db        *sql.DB
	bible     *orthocal.Bible
	useJulian bool
	doJump    bool
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
	var today time.Time
	tz, e := time.LoadLocation("America/Denver")
	if e != nil {
		today = time.Now().In(tz)
	} else {
		today = time.Now()
	}

	Day := orthocal.NewDay(today.Year(), int(today.Month()), today.Day(), self.useJulian, self.doJump, self.db, self.bible)

	writer.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "\t")

	if e = encoder.Encode(Day); e != nil {
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

	Day := orthocal.NewDay(year, month, day, self.useJulian, self.doJump, self.db, self.bible)

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

	writer.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "\t")

	io.WriteString(writer, "[")
	for day := 1; day <= 31; day++ {
		Day := orthocal.NewDay(year, month, day, self.useJulian, self.doJump, self.db, nil)

		// NewDay automatically wraps, so break out once we hit the next month
		if Day.Month != month {
			break
		}

		if day > 1 {
			io.WriteString(writer, ", ")
		}

		e = encoder.Encode(Day)
		if e != nil {
			http.Error(writer, "Server Error", http.StatusInternalServerError)
			log.Printf("Could not marshal json for dayHandler: %#n.", e)
		}
	}
	io.WriteString(writer, "]")
}

func (self *CalendarServer) icalHandler(writer http.ResponseWriter, request *http.Request) {
	var today time.Time
	tz, e := time.LoadLocation("America/Denver")
	if e != nil {
		today = time.Now().In(tz)
	} else {
		today = time.Now()
	}

	writer.Header().Set("Content-Type", "text/calendar")

	fmt.Fprintf(writer, "BEGIN:VCALENDAR\r\nPRODID:-//brianglass//Orthocal//en\r\nVERSION:2.0\r\n")
	for i := 0; i <= 30; i++ {
		date := today.AddDate(0, 0, i)
		day := orthocal.NewDay(date.Year(), int(date.Month()), date.Day(), self.useJulian, self.doJump, self.db, nil)
		uid := date.Format("2006-01-02") + "@orthocal.info"

		fmt.Fprintf(writer, "BEGIN:VEVENT\r\nUID:%s\r\nDTSTAMP:%s\r\nDTSTART:%s\r\nSUMMARY:%s\r\nDESCRIPTION:%s\r\nCLASS:PUBLIC\r\nEND:VEVENT\r\n", uid, today.Format("20060102T150405Z"), date.Format("20060102"), day.Commemorations[0].Title, "This is a test.")
	}

	fmt.Fprintf(writer, "END:VCALENDAR")
}
