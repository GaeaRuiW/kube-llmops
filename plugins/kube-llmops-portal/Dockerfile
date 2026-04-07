FROM node:22-alpine AS build
WORKDIR /plugin
COPY package.json package-lock.json* ./
RUN npm install
COPY . .
RUN npx @kinvolk/headlamp-plugin build
RUN mkdir -p /plugins/kube-llmops-portal && \
    cp dist/main.js package.json /plugins/kube-llmops-portal/

FROM busybox:stable
COPY --from=build /plugins /plugins
