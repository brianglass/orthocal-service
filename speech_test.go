package main

import (
	"database/sql"
	alexa "github.com/brianglass/go-alexa/skillserver"
	"github.com/brianglass/orthocal"
	_ "github.com/mattn/go-sqlite3"
	"os"
	"strings"
	"testing"
)

func TestEstimateGroupSize(t *testing.T) {
	db, e := sql.Open("sqlite3", "kjv.db")
	if e != nil {
		t.Errorf("Got error opening database: %#v.", e)
	}
	bible := orthocal.NewBible(db)

	testCases := []struct {
		reference string
		groupSize int
	}{
		{"Matt 22.15-17", -1},
		{"Matt 22.15-23.39", 36},
		{"Wis 4, 6, 7, 2", 50},
		{"Matt 1:1-7:8", 60},
	}

	for _, tc := range testCases {
		t.Run("Estimate Passage", func(t *testing.T) {
			passage := bible.Lookup(tc.reference)
			groupSize := EstimateGroupSize(passage)
			if groupSize != tc.groupSize {
				t.Errorf("groupSize should be %d but is %d", tc.groupSize, groupSize)
			}
		})
	}
}

func TestDaySpeechNoTitles(t *testing.T) {
	db, e := sql.Open("sqlite3", "oca_calendar.db")
	if e != nil {
		os.Exit(1)
	}
	defer db.Close()
	factory := orthocal.NewDayFactory(false, false, db)
	// This day has no titles
	day := factory.NewDay(2019, 2, 11, nil)

	builder := alexa.NewSSMLTextBuilder()
	card := DaySpeech(builder, day, TZ)
	if !strings.HasPrefix(card, "No Fast") {
		t.Errorf("Card should start with fasting information, but doesn't.\n")
	}
}
