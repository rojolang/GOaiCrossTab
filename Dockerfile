
# Start from the latest golang base image
FROM golang:latest

# Add Maintainer Info
LABEL maintainer="admin@cntrl.ai"

# Set the Current Working Directory inside the container
WORKDIR /app

# Install git.
RUN apt-get update && apt-get install -y git

COPY . .

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
RUN go mod download

# Build the Go app
RUN go build -o main .

# Expose port 8080 to the outside world
EXPOSE 8080

# Command to run the executable
CMD ["/app/main"]