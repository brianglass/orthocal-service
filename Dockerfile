# Use a multi-stage build to make our final image small.

# We build the binary using the standard golang image
FROM brianglass/golang-sqlite:latest as builder

WORKDIR /go/src/github.com/brianglass/orthocal-service
ADD . .
RUN go get -v -d ./...
RUN go build -v -o orthocal-service .

# The final image uses Alpine Linux to avoid all the baggage

FROM alpine:latest

WORKDIR /root

# Need tzdata to be able to load timeszones
# need the CA certs to be able to make secure requests to Amazon
RUN apk --no-cache add tzdata ca-certificates

COPY --from=builder /go/src/github.com/brianglass/orthocal/*.db ./
COPY --from=builder /go/src/github.com/brianglass/english_bible/bible.db ./english.db
COPY --from=builder /go/src/github.com/brianglass/orthocal-service/orthocal-service ./

EXPOSE 8080
ENTRYPOINT ./orthocal-service
