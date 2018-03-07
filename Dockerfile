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

COPY --from=builder /go/src/github.com/brianglass/orthocal/*.db ./
COPY templates ./templates
COPY --from=builder /go/src/github.com/brianglass/orthocal-service/orthocal-service ./

EXPOSE 8080
ENTRYPOINT ./orthocal-service
