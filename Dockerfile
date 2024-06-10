FROM golang:1.22-bullseye

# Set destination for COPY
WORKDIR /app

# Download Go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code. Note the slash at the end, as explained in
# https://docs.docker.com/engine/reference/builder/#copy
COPY *.go ./
COPY auth ./auth 
COPY model ./model 

# Build
RUN GOOS=linux go build -o /server-bin

EXPOSE 5000
CMD ["/server-bin"]
