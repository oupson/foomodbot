FROM golang:1.24-alpine as build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/foomodbot

FROM alpine:latest
WORKDIR /app
RUN adduser -D foomodbot
COPY --from=build /app/foomodbot /app/foomodbot
USER foomodbot:foomodbot
ENTRYPOINT ["/app/foomodbot"]
