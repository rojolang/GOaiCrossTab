FROM golang:latest

LABEL authors="rojo"

# Set the Current Working Directory inside the container
WORKDIR /app

# Install git.
RUN apt-get update && apt-get install -y git

# Clone the repository
RUN git clone https://github.com/rojolang/GOaiCrossTab.git .

# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
RUN go mod download

# Accept the Google service key as a build argument
ARG GOOGLE_SERVICE_KEY

# If the Google service key is provided, convert it to base64 and store it in a file
RUN if [ -n "$GOOGLE_SERVICE_KEY" ]; then echo $GOOGLE_SERVICE_KEY | base64 > key.txt; fi

# Build the Go app
RUN go build -o main .

# Expose port 8080 to the outside world
EXPOSE 8080

# Command to run the executable
CMD ["/app/main"]