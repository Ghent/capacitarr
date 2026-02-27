# Capacitarr Implementation Plan

Capacitarr is a standalone capacity manager application combining a highly efficient Go backend with a premium Vue.js & NuxtUI frontend, utilizing SQLite for zero-config data persistence.

## Proposed Architecture Updates
We will ensure that the Go backend and Vue frontend are heavily decoupled logically but operate together harmoniously in our monorepo. Crucially, the routing layers on both the backend and frontend will be built to respect a configurable `base_url` parameter to ensure seamless reverse proxying in sub-directories, as well as native subdomain reverse proxy handling.

---

## Technical Action Plan

### Phase 1: Project Initialization & Licensing
- [x] Configure git repository tracking
- [ ] Acquire and add PolyForm Noncommercial `LICENSE`
- [ ] Add `CONTRIBUTING.md` outlining CLA requirements
- [ ] Initialize Go module for the backend
- [ ] Initialize Vue.js with NuxtUI for the frontend scaffolding
- [ ] Initial scaffolding commit

### Phase 2: Core Backend Engine (Go + SQLite)
- [ ] Scaffold Go application structure and logging
- [ ] Implement central configuration manager (handling `base_url` for sub-directory proxies)
- [ ] Initialize SQLite database using GORM
- [ ] Create `library_history` and `auth_configs` table schemas and run auto-migrations
- [ ] Set up basic HTTP router structure (e.g., Echo or standard library with routing features mapping seamlessly to `base_url`)
- [ ] Implement JWT form authentication logic for web UI sessions
- [ ] Implement API key generation and validation middleware for programmatic endpoints

### Phase 3: Data Aggregation & Logic
- [ ] Implement simulated/actual data polling routines (based on capacity limits)
- [ ] Implement background TTL chron jobs for data preservation:
  - Snapshots kept hourly (7 days)
  - Roll up into daily averages (30 days)
  - Roll up into weekly averages (up to 1 year)
  - Prune data strictly older than 1 year
- [ ] Expose REST API endpoints to fetch capacity trend metrics

### Phase 4: Frontend Foundation (Vue + NuxtUI)
- [ ] Configure Vue Router to dynamically respect the backend-injected or environment-level `base_url` for asset and page routing
- [ ] Build core app layout structure (Dashboard shell) and Login View
- [ ] Connect form authentication to Go backend and securely store/manage JWTs
- [ ] Implement NuxtUI for premium dark/light mode toggle utilizing Slate/Zinc palettes
- [ ] Connect Vue application to Go REST API endpoints via robust asynchronous service layer

### Phase 5: Premium Data Visualization & Polish
- [ ] Integrate **ApexCharts** into frontend components
- [ ] Build capacity trend dashboards with premium micro-interactions and smooth loading states
- [ ] Comprehensive responsiveness testing for mobile/desktop
- [ ] Final UI/UX polish and refinement

### Phase 6: Deployment & Packaging
- [ ] Construct multi-stage `Dockerfile` to compile Vue frontend and Go backend into a single statically linked binary
- [ ] Verify reverse proxy behavior in compiled Docker container environment
