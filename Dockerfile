FROM alpine:3.8

RUN apk add --no-cache ca-certificates

ADD ./ingress-operator /ingress-operator

ENTRYPOINT ["/ingress-operator"]
