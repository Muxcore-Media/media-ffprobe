FROM golang:1.26-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /media-ffprobe ./cmd/module

FROM alpine:3.21
RUN apk add --no-cache ffmpeg
RUN adduser -D -h /data app
USER app
WORKDIR /app
COPY --from=builder /media-ffprobe .
ENTRYPOINT ["./media-ffprobe"]
