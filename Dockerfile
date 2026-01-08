# Build stage for Go application
FROM golang:1.23.4 AS builder

WORKDIR /workspace

# Copy source files first
COPY go.mod go.sum ./
COPY cli cli/
COPY internal internal/
COPY pkg pkg/
COPY public public/

# Download dependencies
ARG GOPROXY
ARG GOSUMDB
RUN go mod download

# Build the application
RUN CGO_ENABLED=0 GOOS=linux GO111MODULE=on go build -a -o q3 ./cli

# Build stage for Quake 3 server
FROM alpine:3.20.3 AS quake-n-bake

# Install build dependencies, clone, build, and cleanup in one layer
RUN apk add --no-cache git gcc make libc-dev cmake
RUN git clone --depth 1 https://github.com/ioquake/ioq3 /ioq3
WORKDIR /ioq3
RUN mkdir build && cd build && \
    cmake -DBUILD_CLIENT=0 -DBUILD_SERVER=1 -DGENERATE_TESTS=0 .. && \
    make && \
    find . -name "ioq3ded*" -type f -exec cp {} /usr/local/bin/ioq3ded \;

# Final production stage

FROM alpine:3.20.3 AS production

# Create non-root user
RUN addgroup -g 1000 q3user && \
    adduser -D -u 1000 -G q3user q3user

# Copy binaries from build stages
COPY --from=builder /workspace/q3 /usr/local/bin/q3
COPY --from=quake-n-bake /usr/local/bin/ioq3ded /usr/local/bin/ioq3ded
COPY --from=quake-n-bake /lib/ld-musl-*.so.1 /lib/

# Set ownership
RUN chown q3user:q3user /usr/local/bin/q3 /usr/local/bin/ioq3ded

# Switch to non-root user
USER q3user

ENTRYPOINT ["/usr/local/bin/q3"]
