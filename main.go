package main

import (
	"database/sql"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
	"html/template"
	"log"
	"net/http"
	"os"
)

func main() {
	var ocadb *sql.DB
	var e error

	if ocadb, e = sql.Open("sqlite3", "oca_calendar.db"); e != nil {
		log.Printf("Got error opening database: %#n. Exiting.", e)
		os.Exit(1)
	}
	defer ocadb.Close()

	router := mux.NewRouter()

	router.HandleFunc("/", homeHandler)

	ocaRoute := router.PathPrefix("/oca")
	NewCalendarServer(ocaRoute, ocadb, false, true)

	router.Use(logHeaderMiddleware)
	router.Use(handlers.CompressHandler)

	http.ListenAndServe(":8080", handlers.CombinedLoggingHandler(os.Stdout, router))
}

func logHeaderMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		log.Println(request.Header)
		next.ServeHTTP(writer, request)
	})
}

func homeHandler(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "text/html")

	t, e := template.ParseFiles("templates/home.html")
	if e != nil {
		http.Error(writer, "Template not found.", http.StatusInternalServerError)
		log.Println(e.Error())
		return
	}

	if e = t.Execute(writer, nil); e != nil {
		http.Error(writer, "Template rendering failed.", http.StatusInternalServerError)
		log.Println(e.Error())
		return
	}
}
