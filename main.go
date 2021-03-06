package main

import (
	"database/sql"
	"github.com/brianglass/english_bible"
	alexa "github.com/brianglass/go-alexa/skillserver"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/cors"
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
		log.Printf("Got error opening database: %#v. Exiting.", e)
		os.Exit(1)
	}
	defer ocadb.Close()

	if bibledb, e = sql.Open("sqlite3", "english.db"); e != nil {
		log.Printf("Got error opening database: %#v. Exiting.", e)
		os.Exit(1)
	}
	defer bibledb.Close()

	bible := english_bible.NewBible(bibledb)

	// Setup HTTP routers

	router := mux.NewRouter()

	// Google health check expects to to receive a response for /. Do not delete.
	router.HandleFunc("/", healthHandler)
	router.HandleFunc("/healthz", healthHandler)

	ocaRouter := router.PathPrefix("/api/oca").Subrouter()
	NewCalendarServer(ocaRouter, ocadb, false, true, bible, "OCA")

	rocorRouter := router.PathPrefix("/api/rocor").Subrouter()
	NewCalendarServer(rocorRouter, ocadb, true, true, bible, "ROCOR") // Apparently Rocor now does the Lukan jump

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

func healthHandler(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "text/plain")
	writer.WriteHeader(http.StatusOK)
	writer.Write([]byte("ok"))
}

func logHeaderMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		log.Println(request.Header)
		next.ServeHTTP(writer, request)
	})
}
