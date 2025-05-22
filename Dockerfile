
FROM golang:1.20-alpine AS builder


RUN apk add --no-cache git ca-certificates tzdata


WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download


COPY . .


RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main cmd/api-gateway/main.go

FROM alpine:latest


RUN apk --no-cache add ca-certificates tzdata wget


RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

WORKDIR /root/


COPY --from=builder /app/main .

RUN chown appuser:appgroup /root/main


USER appuser


EXPOSE 8080

CMD ["./main", "-addr=:8080"]
