FROM docker.io/skpr/node:24-v3-latest

USER root

RUN apk add --no-cache g++ make python3 nodejs npm git gcompat github-cli linux-headers openssh-client \
 && npm i -g t3 node-gyp \
 && FFI_VER=$(node -e "console.log(require('/usr/local/lib/node_modules/t3/node_modules/ffi-rs/package.json').version)") \
 && npm i -g --force "@yuuang/ffi-rs-linux-x64-musl@${FFI_VER}" \
 && (cd /usr/local/lib/node_modules/t3/node_modules/node-pty && node-gyp rebuild) \
 && node -e "require('/usr/local/lib/node_modules/t3/node_modules/ffi-rs'); console.log('ffi-rs OK')" \
 && node -e "require('/usr/local/lib/node_modules/t3/node_modules/node-pty'); console.log('node-pty OK')"

USER skpr

RUN mkdir -p /home/skpr/.ssh && \
    chown skpr:skpr /home/skpr/.ssh && \
    ssh-keyscan github.com >> /home/skpr/.ssh/known_hosts && \
    chmod 644 /home/skpr/.ssh/known_hosts

ENV T3CODE_MODE=web \
    T3CODE_HOST=0.0.0.0 \
    T3CODE_NO_BROWSER=true \
    T3CODE_AUTO_BOOTSTRAP_PROJECT_FROM_CWD=false

WORKDIR /workspace
EXPOSE 3773

ENTRYPOINT ["t3"]
