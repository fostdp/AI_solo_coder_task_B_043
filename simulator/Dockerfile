FROM golang:1.21-alpine

WORKDIR /app

COPY go.mod go.sum* ./
RUN go mod download || true

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o simulator .

CMD ["./simulator"]
