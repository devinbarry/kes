# The builder is the reviewed Go image mirrored in our registry, pinned by digest.
ARG GO_IMAGE=example.invalid/mirror/library/golang:1.27.1-alpine3.24-amd64@sha256:cd9a32216aee5667f957a62d13a10032a63fd58e14b3f3d9cc8c2122f501e95e

FROM ${GO_IMAGE} AS build
ENV CGO_ENABLED=0 GOTOOLCHAIN=local GOFLAGS=-mod=readonly
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY . .
ARG SOURCE_COMMIT
# A release tag such as v2.0.0; untagged builds report the commit instead.
ARG VERSION
# Tests are offline: servers listen on loopback and the Vault integration test is skipped.
RUN case "$SOURCE_COMMIT" in *[!0-9a-f]*|'') echo "SOURCE_COMMIT must be a commit SHA" >&2; exit 2 ;; esac \
 && version="${VERSION:-$SOURCE_COMMIT}" \
 && case "$version" in "$SOURCE_COMMIT"|v[0-9]*.[0-9]*.[0-9]*) ;; *) echo "VERSION must be a vX.Y.Z tag" >&2; exit 2 ;; esac \
 && case "$version" in *[!0-9a-zA-Z.-]*) echo "VERSION must be a vX.Y.Z tag" >&2; exit 2 ;; esac \
 && test -z "$(gofmt -l .)" \
 && go vet ./... \
 && go test -count=1 -timeout=180s ./... \
 && go build -trimpath -buildvcs=false \
      -ldflags "-s -w -X github.com/minio/kes/internal/sys.version=${version} -X github.com/minio/kes/internal/sys.commitID=${SOURCE_COMMIT}" \
      -o /out/kes ./cmd/kes

# No shell, package manager or system CA roots: KES trusts only the CA files its config names.
FROM scratch
COPY LICENSE /LICENSE
COPY --from=build /out/kes /kes
USER 65532:65532
EXPOSE 7373
ENTRYPOINT ["/kes"]
CMD ["server", "--config", "/etc/kes/config.yaml"]
