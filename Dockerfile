FROM --platform=$BUILDPLATFORM golang:1.23 as builder
ARG TARGETPLATFORM
ARG ENVVAR=CGO_ENABLED=0
WORKDIR /go/src/github.com/atlassian/escalator/
COPY go.mod go.sum Makefile ./
COPY cmd cmd
COPY pkg pkg
RUN make build ENVVAR=$ENVVAR

FROM alpine:3.16
RUN apk --no-cache add ca-certificates
COPY --from=builder /go/src/github.com/atlassian/escalator/escalator ./main
CMD [ "./main" ]
