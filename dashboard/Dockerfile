# Stage 1: Build frontend
FROM node:20-alpine AS frontend
WORKDIR /app/web
COPY dashboard/web/package.json dashboard/web/package-lock.json ./
RUN npm ci
COPY dashboard/web/ .
RUN npm run build

# Stage 2: Build backend
FROM golang:1.24-alpine AS backend
RUN apk add --no-cache git
WORKDIR /app
# Copy operator module (needed for CRD types)
COPY operator/go.mod operator/go.sum ./operator/
COPY operator/api/ ./operator/api/
# Copy dashboard module
COPY dashboard/go.mod dashboard/go.sum ./dashboard/
WORKDIR /app/dashboard
RUN go mod download
COPY dashboard/ .
# Copy frontend build into embed location
COPY --from=frontend /app/web/dist ./web/dist
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /dashboard .

# Stage 3: Final image
FROM gcr.io/distroless/static:nonroot
COPY --from=backend /dashboard /dashboard
USER nonroot:nonroot
EXPOSE 3000
ENTRYPOINT ["/dashboard"]
