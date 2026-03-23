FROM golang:1.23-alpine

WORKDIR /app

copy . .

RUN go build -o main

CMD ["./main"]