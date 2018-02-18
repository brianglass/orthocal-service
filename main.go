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
)

func main() {
	ocadb, e := sql.Open("sqlite3", "oca_calendar.db")
	if e != nil {
		log.Printf("Got error opening database: %#n. Exiting.", e)
		os.Exit(1)
	}

	ocaRouter := router.PathPrefix("/oca/")
	NewCalendarServer(ocaRouter, ocadb, false, true)

	router.Use(handlers.CompressHandler)

	http.ListenAndServe(":8080", handlers.CombinedLoggingHandler(os.Stdout, router))
}
