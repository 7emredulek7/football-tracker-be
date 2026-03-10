FROM golang:alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o football-tracker-be .

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/football-tracker-be .

EXPOSE 8080

CMD ["./football-tracker-be"]
