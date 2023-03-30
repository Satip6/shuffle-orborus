# Start from a Golang base image
FROM golang:latest

# Set the working directory to /app
WORKDIR /app

# Copy the current directory contents into the container at /app
COPY . /app

# Build the Go app
RUN go build -o test .

# Expose port 8080 for the application
EXPOSE 80

# Run the application when the container starts
CMD ["./test"]