FROM alpine:3.24

RUN apk add --no-cache \
    bash \
    curl \
    git \
    nodejs \
    libstdc++ \
    libgcc \
    docker-cli

# Consistent UID between container and workstation.
RUN adduser -D -u 1000 skpr

USER skpr

# Create the config dirs as skpr first; otherwise ADD/COPY below creates the
# parent directories as root, blocking later writes by the skpr user.
RUN mkdir -p \
    /home/skpr/.config/opencode/tools \
    /home/skpr/.config/opencode/agents \
    /home/skpr/.config/opencode/command

# Clone ponytail at a pinned release so the plugin, hooks, and skills are
# all present at their expected relative paths.
RUN git clone --depth 1 --branch v4.7.0 \
    https://github.com/DietrichGebert/ponytail.git \
    /home/skpr/ponytail

# Wire the ponytail plugin into OpenCode's config and expose its slash commands
# globally (opencode discovers commands from ~/.config/opencode/command/).
COPY --chown=skpr:skpr opencode.json /home/skpr/.config/opencode/opencode.json
RUN cp /home/skpr/ponytail/.opencode/command/*.md /home/skpr/.config/opencode/command/

# Pre-create the XDG data/state dirs owned by skpr so that when a named volume
# is mounted at ~/.local/share/opencode, Docker doesn't create the parent
# ~/.local as root and block writes to sibling dirs like ~/.local/state.
RUN mkdir -p /home/skpr/.local/share/opencode /home/skpr/.local/state

RUN curl -fsSL https://opencode.ai/install | bash

ENV PATH="/home/skpr/.opencode/bin:${PATH}"

# The opencode installer only drops the binary in ~/.opencode/bin; the runtime
# dependencies (including @opencode-ai/plugin) are installed into
# ~/.opencode/node_modules on first run. Invoke opencode once at build time so
# that install happens now rather than on container start.
RUN opencode --version

# The custom bash tool imports "@opencode-ai/plugin". Bun resolves modules by
# walking up from the tool file, so make the bundled plugin (installed under
# ~/.opencode/node_modules above) resolvable from ~/.config/opencode/tools/ by
# symlinking it into a node_modules on that path.
RUN mkdir -p /home/skpr/.config/opencode/node_modules/@opencode-ai \
    && ln -s /home/skpr/.opencode/node_modules/@opencode-ai/plugin \
        /home/skpr/.config/opencode/node_modules/@opencode-ai/plugin

CMD ["opencode", "web", "--hostname", "0.0.0.0", "--port", "4096"]
