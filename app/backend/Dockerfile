FROM golang:1.27-alpine

WORKDIR /app/backend

COPY go.mod .

RUN go mod tidy

COPY . .

ENV PG_URL=postgresql://user:password@db:5432/web_monitor_db?sslmode=disable

# Install make
RUN apk add --no-cache make

RUN make build

EXPOSE 8080

ENTRYPOINT ["./bin/main"]
