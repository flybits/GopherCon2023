# BUILD STAGE
FROM golang:1.20-alpine
ARG GITHUB_SSH_KEY

RUN apk add --no-cache git openssh && \
    mkdir /root/.ssh && \
        echo "$GITHUB_SSH_KEY" > /root/.ssh/id_rsa && \
        chmod 400 /root/.ssh/id_rsa && \
        touch /root/.ssh/known_hosts && \
        ssh-keyscan github.com >> /root/.ssh/known_hosts && \
        git config --global url.git@github.com:.insteadOf https://github.com/

# Set the working directory
WORKDIR /app

# Copy the Go modules manifests
COPY go.mod go.sum ./

# Download and cache Go dependencies
RUN go mod download

# Copy the source code into the container
COPY . .

# Build the Go application
RUN go build -o main .

# Expose port 8080 for the application
EXPOSE 8000 8001

# Set the entry point command to run the application
CMD ["./main"]