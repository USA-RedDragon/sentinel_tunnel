FROM scratch

COPY --from=alpine:3.20 /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

COPY sentinel_tunnel /

ENTRYPOINT ["/sentinel_tunnel"]
