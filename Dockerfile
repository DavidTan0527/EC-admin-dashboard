FROM golang:1.22.4-bullseye

ENV GO111MODULE=on
ENV GOFLAGS=-mod=mod

# Set destination for COPY
WORKDIR /app

# Download Go modules
COPY go.mod go.sum ./
RUN go mod vendor
RUN go mod download

# Copy the source code. Note the slash at the end, as explained in
# https://docs.docker.com/engine/reference/builder/#copy
COPY *.go ./

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o /server-bin

EXPOSE 5000
CMD ["/server-bin"]
