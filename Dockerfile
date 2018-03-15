FROM alpine:3.7

RUN apk add --no-cache ca-certificates

ADD ./ingress-operator /ingress-operator

ENTRYPOINT ["/ingress-operator"]
