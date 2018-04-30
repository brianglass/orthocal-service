package main

import (
	"database/sql"
	"github.com/brianglass/orthocal"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
	alexa "github.com/mikeflynn/go-alexa/skillserver"
	"github.com/rs/cors"
	"html/template"
	"log"
	"net/http"
	"os"
	"time"
)

const (
	// This is about the middle of the country including Hawaii and Alaska
	// Folks on the east coast won't much care after midnight and the folks in
	// Hawaii will have the day change happen at 9pm. That's not ideal, but my
	// guess is that most people won't be using the service after 9pm.
	TimeZone = "America/Los_Angeles"
)

var (
	TZ         *time.Location
	AlexaAppId = os.Getenv("ALEXA_APP_ID")
)

func init() {
	var e error
	TZ, e = time.LoadLocation(TimeZone)
	if e != nil {
		TZ = time.UTC
		log.Printf("Error loading '%s' timezone, using UTC.", TimeZone)
	}
}

func main() {
	var ocadb, bibledb *sql.DB
	var e error

	// Open up all the requisite databases

	if ocadb, e = sql.Open("sqlite3", "oca_calendar.db"); e != nil {
		log.Printf("Got error opening database: %#n. Exiting.", e)
		os.Exit(1)
	}
	defer ocadb.Close()

	if bibledb, e = sql.Open("sqlite3", "kjv.db"); e != nil {
		log.Printf("Got error opening database: %#n. Exiting.", e)
		os.Exit(1)
	}
	defer bibledb.Close()

	bible := orthocal.NewBible(bibledb)

	// Setup HTTP routers

	router := mux.NewRouter()

	// Google health check expects to to receive a response for /. Do not delete.
	router.HandleFunc("/", homeHandler)

	ocaRouter := router.PathPrefix("/api/oca").Subrouter()
	NewCalendarServer(ocaRouter, ocadb, false, true, bible)

	rocorRouter := router.PathPrefix("/api/rocor").Subrouter()
	NewCalendarServer(rocorRouter, ocadb, true, true, bible) // Apparently Rocor now does the Lukan jump

	// Setup Alexa skill

	apps := map[string]interface{}{
		"/echo/": NewSkill(AlexaAppId, ocadb, false, true, bible, TZ),
	}
	alexa.Init(apps, router.NewRoute().Subrouter())

	// Setup middleware

	router.Use(cors.Default().Handler)
	router.Use(handlers.CompressHandler)
	router.Use(logHeaderMiddleware)

	// Launch the HTTP server

	http.ListenAndServe(":8080", handlers.CombinedLoggingHandler(os.Stdout, router))
}

func homeHandler(writer http.ResponseWriter, request *http.Request) {
	t, e := template.ParseFiles("templates/home.html")
	if e != nil {
		http.Error(writer, "Template not found.", http.StatusInternalServerError)
		log.Println(e.Error())
		return
	}

	writer.Header().Set("Content-Type", "text/html")
	if e = t.Execute(writer, nil); e != nil {
		http.Error(writer, "Template rendering failed.", http.StatusInternalServerError)
		log.Println(e.Error())
		return
	}
}

func logHeaderMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		log.Println(request.Header)
		next.ServeHTTP(writer, request)
	})
}
