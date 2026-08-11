# Blog System

This repository contains the complete blog application as a monorepo.

## Structure

- `frontend/`: Vue 3 and Vite web application.
- `backend/`: Go, Gin, and GORM API service.
- `deploy/`: Production deployment configuration.

## Local development

Start the backend from `backend/`:

```bash
go run ./cmd/server
```

Start the frontend from `frontend/`:

```bash
npm install
npm run dev
```

The Vite development server proxies `/api` requests to the backend at
`http://localhost:8080`.
