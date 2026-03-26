FROM golang:1.25-alpine

WORKDIR /app

RUN apk add --no-cache wget curl

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN go build -o main ./cmd/api

EXPOSE 8000

CMD ["./main"]