FROM alpine:3.23

RUN apk add --no-cache \
    bash \
    curl \
    git \
    nodejs \
    libstdc++ \
    libgcc

# Consistent UID between container and workstation.
RUN adduser -D -u 1000 pinchy

# Install the pinchy CLI (pre-built by GoReleaser, copied into build context root).
COPY pinchy /usr/local/bin/pinchy

USER pinchy

# Create the config dirs as pinchy first; otherwise ADD/COPY below creates the
# parent directories as root, blocking later writes by the pinchy user.
RUN mkdir -p \
    /home/pinchy/.config/opencode/tools \
    /home/pinchy/.config/opencode/agents \
    /home/pinchy/.config/opencode/command

# Override the built-in bash tool so commands run in the remote Kubernetes environment.
COPY --chown=pinchy:pinchy images/opencode/tools/bash.ts /home/pinchy/.config/opencode/tools/bash.ts

# Custom agents loaded by opencode from ~/.config/opencode/agents/.
COPY --chown=pinchy:pinchy images/opencode/agents/ /home/pinchy/.config/opencode/agents/

# Clone ponytail at a pinned release so the plugin, hooks, and skills are
# all present at their expected relative paths.
RUN git clone --depth 1 --branch v4.7.0 \
    https://github.com/DietrichGebert/ponytail.git \
    /home/pinchy/ponytail

# Wire the ponytail plugin into OpenCode's config and expose its slash commands
# globally (opencode discovers commands from ~/.config/opencode/command/).
COPY --chown=pinchy:pinchy images/opencode/opencode.json /home/pinchy/.config/opencode/opencode.json
RUN cp /home/pinchy/ponytail/.opencode/command/*.md /home/pinchy/.config/opencode/command/

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

# The custom bash tool imports "@opencode-ai/plugin". Bun resolves modules by
# walking up from the tool file, so make the bundled plugin (installed under
# ~/.opencode/node_modules above) resolvable from ~/.config/opencode/tools/ by
# symlinking it into a node_modules on that path.
RUN mkdir -p /home/pinchy/.config/opencode/node_modules/@opencode-ai \
    && ln -s /home/pinchy/.opencode/node_modules/@opencode-ai/plugin \
        /home/pinchy/.config/opencode/node_modules/@opencode-ai/plugin

CMD ["opencode", "web", "--hostname", "0.0.0.0", "--port", "4096"]
