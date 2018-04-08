package main

import (
	"database/sql"
	"github.com/brianglass/orthocal"
	_ "github.com/mattn/go-sqlite3"
	"testing"
)

func TestEstimateGroupSize(t *testing.T) {
	db, e := sql.Open("sqlite3", "kjv.db")
	if e != nil {
		t.Errorf("Got error opening database: %#n.", e)
	}
	bible := orthocal.NewBible(db)

	testCases := []struct {
		reference  string
		groupCount int
		groupSize  int
	}{
		{"Matt 22.15-17", 1, -1},
		{"Matt 22.15-23.39", 2, 36},
		{"Wis 4, 6, 7, 2", 2, 50},
		{"Matt 1:1-7:8", 3, 60},
	}

	for _, tc := range testCases {
		t.Run("Estimate Passage", func(t *testing.T) {
			passage := bible.Lookup(tc.reference)
			groupCount, groupSize := EstimateGroupSize(passage)
			if groupCount != tc.groupCount {
				t.Errorf("groupCount should be %d but is %d", tc.groupCount, groupCount)
			}
			if groupSize != tc.groupSize {
				t.Errorf("groupSize should be %d but is %d", tc.groupSize, groupSize)
			}
		})
	}
}
