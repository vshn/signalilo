# Authors:
# Simon Gerber <simon.gerber@vshn.ch>
#
# License:
# Copyright (c) 2019, VSHN AG, <info@vshn.ch>
# Licensed under "BSD 3-Clause". See LICENSE file.

#####################
# STEP 1 build binary
#####################
FROM golang:alpine as builder

# Prepare needed packages for building
RUN apk update && apk add --no-cache git ca-certificates bzr && \
    adduser -D -g '' appuser

# Workdir must be outside of GOPATH because of go mod usage
WORKDIR /src/signalilo

# Download modules for leveraging docker build cache
COPY go.mod go.sum ./
RUN go mod download

# Add code and build app
COPY . .
RUN CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64 \
    go build -a -installsuffix cgo -ldflags="-w -s" -o /go/bin/signalilo

# Run tests
RUN CGO_ENABLED=0 go test

############################
# STEP 2 build runtime image
############################
FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /go/bin/signalilo /go/bin/signalilo

USER appuser
EXPOSE 8888

ENTRYPOINT ["/go/bin/signalilo"]
