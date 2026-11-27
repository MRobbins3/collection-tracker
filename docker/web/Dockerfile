# Dev image for the web service: Nuxt runs on Node (ADR 0005), Bun is the
# package manager. The oven/bun image has no Node, so Nuxt would run on
# Bun's runtime, which lacks some Node APIs (e.g. node:module findPackageJSON
# used by @vite-pwa/nuxt 2). Keep the Node major in step with .nvmrc and the
# Bun version in step with ci.yml's bun-version (see Documentation/dependencies.md).
FROM oven/bun:1.4.5 AS bun

FROM node:26.13.0-slim
COPY --from=bun /usr/local/bin/bun /usr/local/bin/bun
RUN ln -s /usr/local/bin/bun /usr/local/bin/bunx
WORKDIR /app
