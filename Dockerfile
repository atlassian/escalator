FROM golang:1.11 as builder
WORKDIR /go/src/github.com/atlassian/escalator/
RUN curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh
COPY Gopkg.toml Gopkg.lock Makefile ./
RUN make setup
COPY cmd cmd
COPY pkg pkg
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo cmd/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates 
COPY --from=builder /go/src/github.com/atlassian/escalator/main .
CMD [ "./main" ]
