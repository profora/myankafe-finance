FROM alpine:3.20

# The GHCR Pressly image cannot be pulled anonymously. Use the published
# static release binary instead, pinned to the same Goose version as CI.
ARG GOOSE_VERSION=3.24.3
ARG TARGETARCH
RUN apk add --no-cache ca-certificates curl \
	&& case "$TARGETARCH" in \
		amd64) file=goose_linux_x86_64; sum=93255d4bdb085a75f0ed1b07e5fa743ff09d8c6fa50a526b3949fe0d76f3ef81 ;; \
		arm64) file=goose_linux_arm64; sum=70ca8833cc6ef94725ca6d349491cc010d015c5f5b3c36bf355857bdb9ae7acf ;; \
		*) echo "unsupported architecture: $TARGETARCH" >&2; exit 1 ;; \
	esac \
	&& curl -fsSL -o /tmp/goose "https://github.com/pressly/goose/releases/download/v${GOOSE_VERSION}/${file}" \
	&& echo "${sum}  /tmp/goose" | sha256sum -c - \
	&& mv /tmp/goose /usr/local/bin/goose \
	&& chmod 755 /usr/local/bin/goose

ENTRYPOINT ["goose"]
