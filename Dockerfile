# Build stage
FROM golang:1.24-alpine AS builder
WORKDIR /app

ARG TARGETARCH

# Install git for go modules and ca-certificates
RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Use TARGETARCH so buildx can build multi-arch binaries
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -ldflags='-s -w' -o /json_compare

# Final stage
FROM scratch
COPY --from=builder /json_compare /json_compare
# copy frontend static files into the final image so FileServer("./frontend") can serve them
COPY --from=builder /app/frontend /frontend
# Include CA certs for HTTPS if needed
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
EXPOSE 8090
ENTRYPOINT ["/json_compare"]
