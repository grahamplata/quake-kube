# Build stage for Go application
FROM golang:1.24.0-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git

WORKDIR /workspace

# Optimize caching: download dependencies first
COPY go.mod go.sum ./
RUN go mod download

# Copy source and generate static files
COPY . .
RUN go run ./tools/genstatic.go public public

# Build a static Go binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -o q3 ./cli

# Build stage for Quake 3 server
FROM alpine:3.20.3 AS quake-n-bake

RUN apk add --no-cache git gcc g++ make cmake libc-dev sdl2-dev curl-dev
WORKDIR /ioq3
RUN git clone --depth 1 https://github.com/ioquake/ioq3 .
RUN cmake -S . -B build \
    -DCMAKE_BUILD_TYPE=Release \
    -DBUILD_SERVER=ON \
    -DBUILD_CLIENT=OFF \
    -DBUILD_RENDERER_OPENGL1=OFF \
    -DBUILD_RENDERER_OPENGL2=OFF \
    -DBUILD_GAME_LIBRARIES=OFF \
    -DBUILD_GAME_QVMS=OFF \
    -DBUILD_STANDALONE=ON
RUN cmake --build build
RUN cp build/ioq3ded /usr/local/bin/ioq3ded || cp build/Release/ioq3ded /usr/local/bin/ioq3ded

# Final production stage
FROM alpine:3.20.3 AS production

# Install runtime dependencies (ca-certificates for HTTPS downloads)
RUN apk add --no-cache ca-certificates libcurl sdl2

# Create non-root user
RUN addgroup -g 1000 q3user && \
    adduser -D -u 1000 -G q3user q3user

# Copy binaries from build stages
COPY --from=builder /workspace/q3 /usr/local/bin/q3
COPY --from=quake-n-bake /usr/local/bin/ioq3ded /usr/local/bin/ioq3ded

# Set ownership
RUN chown q3user:q3user /usr/local/bin/q3 /usr/local/bin/ioq3ded

WORKDIR /home/q3user
USER q3user

ENTRYPOINT ["/usr/local/bin/q3"]
