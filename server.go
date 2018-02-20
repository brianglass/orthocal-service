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

type CalendarServer struct {
	db        *sql.DB
	useJulian bool
	doJump    bool
}

func NewCalendarServer(router *mux.Route, db *sql.DB, useJulian, doJump bool) *CalendarServer {
	var self CalendarServer

	self.db = db
	self.useJulian = useJulian
	self.doJump = doJump

	r := router.Methods("GET").Subrouter()

	r.HandleFunc("/", self.todayHandler)
	r.HandleFunc("/{year}/{month}/", self.monthHandler)
	r.HandleFunc("/{year}/{month}/{day}/", self.dayHandler)

	return &self
}

func (self *CalendarServer) todayHandler(writer http.ResponseWriter, request *http.Request) {
	today := time.Now()
	log.Printf("Processing today: %s", today)
	Day := orthocal.NewDay(today.Year(), int(today.Month()), today.Day(), self.useJulian, self.doJump, self.db)

	writer.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "\t")

	e := encoder.Encode(Day)
	if e != nil {
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

	Day := orthocal.NewDay(year, month, day, self.useJulian, self.doJump, self.db)

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
		Day := orthocal.NewDay(year, month, day, self.useJulian, self.doJump, self.db)

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
