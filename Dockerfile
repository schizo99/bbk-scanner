FROM golang:1.15 as builder
LABEL maintainer="schizo99@gmail.com"

RUN wget https://frontend.bredbandskollen.se/download/bbk_cli_linux_amd64-1.0 -O /bbk_cli && chmod +x /bbk_cli


COPY . $GOPATH/src/mypackage/myapp/
WORKDIR $GOPATH/src/mypackage/myapp/


# Create appuser.
#RUN adduser --disabled-password --gecos '' apiuser

# Download all the dependencies
RUN go get -d -v

# Install the package
# RUN go install -v ./...
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /go/bin/bbk_scan

FROM ubuntu
RUN apt-get update && apt-get install -y tzdata
WORKDIR /app/
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /go/bin/bbk_scan /app/bbk_scan
COPY --from=builder /bbk_cli /app/bbk_cli

#USER apiuser

# Run the executable
CMD ["/app/bbk_scan"]