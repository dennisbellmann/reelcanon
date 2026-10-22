FROM node:24-bookworm-slim AS frontend-build
WORKDIR /src
COPY frontend/package*.json ./frontend/
COPY frontend/ ./frontend/
COPY localization/ ./localization/
COPY scripts/localization.mjs ./scripts/localization.mjs
COPY backend/internal/accounts/ ./backend/internal/accounts/
COPY mobile/composeApp/src/commonMain/kotlin/ ./mobile/composeApp/src/commonMain/kotlin/
COPY mobile/composeApp/src/commonMain/composeResources/ ./mobile/composeApp/src/commonMain/composeResources/
COPY mobile/androidApp/src/main/kotlin/ ./mobile/androidApp/src/main/kotlin/
WORKDIR /src/frontend
RUN npm ci && npm run build

FROM golang:1.26.0-bookworm AS backend-build
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/reelcanon-api ./cmd/api \
    && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/reelcanon-migrate ./cmd/migrate \
    && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/reelcanon-import ./cmd/import-canon \
    && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/reelcanon-admin ./cmd/admin

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=backend-build /out/reelcanon-api /app/reelcanon-api
COPY --from=backend-build /out/reelcanon-migrate /app/reelcanon-migrate
COPY --from=backend-build /out/reelcanon-import /app/reelcanon-import
COPY --from=backend-build /out/reelcanon-admin /app/reelcanon-admin
COPY --from=frontend-build /src/frontend/dist/ /app/frontend/
COPY database/migrations/ /app/database/migrations/
COPY data/ /app/data/
ENV API_ADDR=:8080
ENV FRONTEND_DIST=/app/frontend
EXPOSE 8080
ENTRYPOINT ["/app/reelcanon-api"]
