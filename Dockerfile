#
# Base image
#
FROM golang:1.12.5-stretch as base
WORKDIR /src
COPY . .

#
# Image used to run code quality checks
#
FROM base as code-quality
ENV SHELLCHECK_VERSION=v0.6.0
ENV SHFMT_VERSION=v2.6.3
RUN apt-get update && apt-get install -y xz-utils
RUN curl -k https://storage.googleapis.com/shellcheck/shellcheck-${SHELLCHECK_VERSION}.linux.x86_64.tar.xz | \
  tar -Jx -C /tmp && mv /tmp/shellcheck-${SHELLCHECK_VERSION}/shellcheck /bin/shellcheck
RUN curl -L https://github.com/mvdan/sh/releases/download/${SHFMT_VERSION}/shfmt_${SHFMT_VERSION}_linux_amd64 \
  -o /bin/shfmt && \
  chmod +x /bin/shfmt

#
# Build image
#
FROM base as build
RUN apt-get update && apt-get install -y ca-certificates zip
RUN make linux-all

#
# Image used to run snyk
#
FROM base as snyk
RUN curl -sL https://deb.nodesource.com/setup_11.x | bash -
RUN apt-get update && apt-get install -y nodejs
RUN npm install -g snyk

#
# Production image
#
FROM scratch as final
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /src/target/bin/linux/amd64/orgbot /bin/
COPY --from=build /src/target/bin/linux/amd64/orgctl /bin/
CMD ["/bin/orgbot"]
