FROM golang:1.26.2-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o monica-turnos-api ./cmd/api

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/monica-turnos-api .

EXPOSE 8080

CMD ["./monica-turnos-api"]
