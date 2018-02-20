# Use a multi-stage build to make our final image small.

# We build the binary using the standard golang image
FROM golang:alpine as builder

WORKDIR /go/src/github.com/brianglass/orthocal-service
ADD . .

# This is the stuff we need to build go-sqlite3
RUN apk --no-cache add alpine-sdk git sqlite-dev sqlite-libs sqlite

RUN go get -v -d ./...
RUN go build -v -o orthocal-service .

# The final image uses Alpine Linux to avoid all the baggage

FROM alpine:latest

WORKDIR /root

COPY --from=builder /go/src/github.com/brianglass/orthocal/oca_calendar.db .
COPY --from=builder /go/src/github.com/brianglass/orthocal-service/orthocal-service .
COPY templates ./templates

EXPOSE 8080
ENTRYPOINT ./orthocal-service
