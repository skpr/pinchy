FROM golang:1.26-alpine AS build

WORKDIR /app

# Download deps first for layer caching.
COPY go.mod go.sum ./
RUN go mod download

# Source.
COPY . .

# Static binary.
RUN CGO_ENABLED=0 go build \
    -ldflags="-extldflags '-static'" \
    -o /pinchy \
    github.com/skpr/pinchy/cmd/pinchy

FROM alpine:3.23

RUN apk add --no-cache \
    bash \
    curl \
    libstdc++ \
    libgcc

# Consistent UID between container and workstation.
RUN adduser -D -u 1000 pinchy

# Install the pinchy CLI (used by the bash tool override).
COPY --from=build /pinchy /usr/local/bin/pinchy

USER pinchy

# Create the config dir as pinchy first; otherwise the ADD below creates the
# parent directories as root, which blocks later writes (e.g. the plugin
# symlink) by the pinchy user.
RUN mkdir -p /home/pinchy/.config/opencode

ADD --chown=pinchy:pinchy images/opencode/opencode.json /home/pinchy/.config/opencode/opencode.json

# Pre-create the XDG data/state dirs owned by pinchy so that when a named volume
# is mounted at ~/.local/share/opencode, Docker doesn't create the parent
# ~/.local as root and block writes to sibling dirs like ~/.local/state.
RUN mkdir -p /home/pinchy/.local/share/opencode /home/pinchy/.local/state

RUN curl -fsSL https://opencode.ai/install | bash

ENV PATH="/home/pinchy/.opencode/bin:${PATH}"

# The opencode installer only drops the binary in ~/.opencode/bin; the runtime
# dependencies (including @opencode-ai/plugin) are installed into
# ~/.opencode/node_modules on first run. Invoke opencode once at build time so
# that install happens now rather than on container start.
RUN opencode --version

# Override the built-in bash tool so commands run in the remote Kubernetes environment.
COPY --chown=pinchy:pinchy images/opencode/tools/bash.ts /home/pinchy/.config/opencode/tools/bash.ts

# Custom agents (reviewer, architect) loaded by opencode from ~/.config/opencode/agents/.
COPY --chown=pinchy:pinchy images/opencode/agents/ /home/pinchy/.config/opencode/agents/

# The custom bash tool imports "@opencode-ai/plugin". Bun resolves modules by
# walking up from the tool file, so make the bundled plugin (installed under
# ~/.opencode/node_modules above) resolvable from ~/.config/opencode/tools/ by
# symlinking it into a node_modules on that path.
RUN mkdir -p /home/pinchy/.config/opencode/node_modules/@opencode-ai \
    && ln -s /home/pinchy/.opencode/node_modules/@opencode-ai/plugin \
        /home/pinchy/.config/opencode/node_modules/@opencode-ai/plugin

CMD ["opencode", "web", "--hostname", "0.0.0.0", "--port", "4096"]
