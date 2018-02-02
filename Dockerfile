FROM golang:latest as builder
WORKDIR /go/src/github.com/atlassian/escalator/
COPY ./ ./
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo cmd/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates 
COPY --from=builder /go/src/github.com/atlassian/escalator/main .
CMD [ "./main" ]