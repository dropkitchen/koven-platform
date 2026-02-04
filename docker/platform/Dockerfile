FROM golang:1.25-alpine AS builder

WORKDIR /build

COPY platform/go.mod platform/go.sum ./
RUN go mod download

COPY platform/ ./

RUN CGO_ENABLED=0 GOOS=linux go build -o koven-platform .

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /build/koven-platform .

EXPOSE 8080

CMD ["./koven-platform", "-addr", ":8080", "-mqtt-broker", "tcp://mqtt:1883"]
