FROM --platform=$BUILDPLATFORM golang:1.20 as builder
ARG TARGETPLATFORM
WORKDIR /go/src/github.com/atlassian/escalator/
COPY go.mod go.sum Makefile ./
COPY cmd cmd
COPY pkg pkg
RUN make build

FROM alpine:3.16
RUN apk --no-cache add ca-certificates
COPY --from=builder /go/src/github.com/atlassian/escalator/escalator ./main
CMD [ "./main" ]
