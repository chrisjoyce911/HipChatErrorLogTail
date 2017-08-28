FROM scratch
ADD build/ca-bundle.crt /etc/ssl/certs/ca-certificates.crt
ADD build/docker-tailtohip /
ENTRYPOINT ["/docker-tailtohip"]
CMD [""]