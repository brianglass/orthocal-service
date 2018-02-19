package main

import (
	"database/sql"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"net/http"
	"os"
)

var (
	router = mux.NewRouter()
	ocadb  *sql.DB
)

func init() {
	var e error

	if ocadb, e = sql.Open("sqlite3", "oca_calendar.db"); e != nil {
		log.Printf("Got error opening database: %#n. Exiting.", e)
		os.Exit(1)
	}

	if e = ocadb.Ping(); e != nil {
		log.Printf("Got error opening database: %#n. Exiting.", e)
		os.Exit(1)
	}
}

func main() {
	defer ocadb.Close()

	ocaRoute := router.PathPrefix("/oca/")
	NewCalendarServer(ocaRoute, ocadb, false, true)

	router.Use(handlers.CompressHandler)

	http.ListenAndServe(":8080", handlers.CombinedLoggingHandler(os.Stdout, router))
}
