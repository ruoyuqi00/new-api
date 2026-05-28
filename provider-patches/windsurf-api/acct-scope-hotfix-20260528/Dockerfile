FROM ghcr.io/dwgx/windsurf-api:latest

COPY patch-acctfix.js /tmp/patch-acctfix.js

RUN node /tmp/patch-acctfix.js \
  && node --check /app/src/handlers/chat.js \
  && rm /tmp/patch-acctfix.js

LABEL org.opencontainers.image.title="WindsurfAPI acct scope hotfix for sub2api"
LABEL org.opencontainers.image.description="Patches nonStreamResponse sticky session acct scope bug observed in WindsurfAPI 2.0.97 / 41a36b9."
