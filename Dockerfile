FROM golang:1.26-alpine AS builder
COPY core/ /build/core/
COPY media-ffprobe/ /build/media-ffprobe/
WORKDIR /build/media-ffprobe
RUN go mod download && CGO_ENABLED=0 go build -o /media-ffprobe ./cmd/module
FROM alpine:3.21
RUN apk add --no-cache ffmpeg
RUN adduser -D -h /data app
USER app
WORKDIR /app
COPY --from=builder /media-ffprobe .
ENTRYPOINT ["./media-ffprobe"]
