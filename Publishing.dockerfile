FROM golang:1.20 AS builder

# Install upx (upx.github.io) to compress the compiled action
RUN apt-get update && apt-get -y install upx

# Turn on Go modules support and disable CGO
ENV GO111MODULE=on CGO_ENABLED=0

WORKDIR /

# Copy the source code.
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./

# Build
RUN GOOS=linux go build -trimpath -a -ldflags "-s -w -extldflags '-static'" -o /bump-tf-values

# Strip any symbols
RUN strip /bump-tf-values

# Compress the compiled action
RUN upx -q -9 /bump-tf-values

# Use  empty container
FROM scratch

# Copy over SSL certificates and root CA bundle.
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy over the compiled action from the first step
COPY --from=builder /bump-tf-values /bump-tf-values

ENTRYPOINT ["/bump-tf-values"]

# docker build -f -t <username/repo> Publishing.dockerfile .
