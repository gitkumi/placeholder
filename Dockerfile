FROM golang:1.20-alpine

ENV ENVIRONMENT=production
ENV GIN_MODE=release

WORKDIR /app

COPY . .

RUN ls -la

RUN go build -o main

EXPOSE 8080

CMD ["./main"]
