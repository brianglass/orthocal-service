FROM golang:alpine

# This is the stuff we need to build go-sqlite3
RUN apk --no-cache add alpine-sdk git sqlite-dev sqlite-libs sqlite

RUN go get github.com/mattn/go-sqlite3
