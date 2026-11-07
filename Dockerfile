FROM node:22-alpine AS frontend
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM golang:1.26-bookworm AS build
WORKDIR /src
COPY backend/ ./backend/
COPY --from=frontend /src/backend/internal/webui/dist ./backend/internal/webui/dist
WORKDIR /src/backend
RUN CGO_ENABLED=0 go build -o /sto-crm ./cmd/server

FROM gcr.io/distroless/static-debian12
COPY --from=build /sto-crm /sto-crm
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/sto-crm"]