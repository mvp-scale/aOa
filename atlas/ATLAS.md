# Universal Domain Structure

> **Status**: Draft — iterating on domain + term coverage
> **Target**: ~200 domains, 3-6 terms each, unique keywords per domain
> **Design**: Shared keywords allowed across domains. Competition resolves ambiguity.
> **Level**: High enough that a developer would say "I'm working on [domain]"

---

## How to Read This

```
@domain_name — one-line description
  term_1: what this term covers
  term_2: what this term covers
  ...
```

Terms are subcategories within a domain. Keywords (the actual tokens that match) come in the next iteration once we're happy with domain + term structure.

---

## Focus Area 1: Authentication & Identity (~8 domains)

```
@authentication — login flows, credentials, session creation
  login: sign-in flows, credential verification, remember-me
  tokens: JWT, refresh tokens, token rotation, expiry
  oauth: OAuth2 flows, providers, authorization codes, PKCE
  mfa: two-factor, TOTP, SMS verification, recovery codes
  sso: single sign-on, SAML, federation, identity providers

@authorization — access control, permissions, policy enforcement
  rbac: roles, permissions, role assignment, role hierarchy
  policies: policy evaluation, rules, conditions, allow/deny
  scopes: API scopes, token scopes, consent, resource access
  acl: access control lists, ownership, sharing, visibility

@user_management — user CRUD, profiles, account lifecycle
  registration: signup, onboarding, email verification, invite
  profiles: user data, preferences, settings, avatar
  accounts: deactivation, deletion, suspension, password reset
  identity: claims, attributes, identity verification

@session — server-side session state and management
  cookies: session cookies, secure flags, SameSite, HttpOnly
  storage: session store, Redis sessions, memory store
  lifecycle: creation, expiration, renewal, invalidation
  fingerprint: device detection, browser fingerprint, IP tracking

@encryption — cryptographic operations and key management
  hashing: bcrypt, argon2, scrypt, salt, pepper
  symmetric: AES, encryption, decryption, key rotation
  asymmetric: RSA, ECDSA, signing, verification, certificates
  secrets: vault, key management, HSM, environment variables
```

Wait — I had 8 but encryption might fit better under security. Let me continue and we'll adjust.

```
@directory — LDAP, Active Directory, user directories
  ldap: bind, search, entries, DN, attributes
  active_directory: groups, OUs, GPO, domain controllers
  provisioning: SCIM, sync, user lifecycle, deprovisioning
```

**Count: 6 domains**

---

## Focus Area 2: API & Communication (~12 domains)

```
@rest_api — RESTful endpoint design and implementation
  endpoints: routes, handlers, controllers, resource paths
  methods: GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS
  responses: status codes, error bodies, pagination, envelopes
  versioning: URL versioning, header versioning, deprecation

@graphql — GraphQL schema, resolvers, and execution
  schema: type definitions, input types, enums, interfaces, unions
  resolvers: field resolvers, data loaders, N+1 prevention
  operations: queries, mutations, subscriptions
  tooling: codegen, introspection, schema stitching, federation

@websocket — persistent bidirectional connections
  connections: handshake, upgrade, close, reconnection
  messaging: send, receive, broadcast, binary frames
  channels: rooms, topics, pub/sub over socket
  protocols: Socket.IO, WS, STOMP, MQTT

@grpc — protocol buffers and RPC services
  protobuf: message types, enums, oneof, well-known types
  services: unary, server streaming, client streaming, bidirectional
  interceptors: middleware, auth, logging, retry
  codegen: protoc, language plugins, generated stubs

@http — HTTP client/server fundamentals
  client: requests, timeouts, retries, connection pooling
  server: listeners, handlers, TLS, keep-alive
  headers: content-type, authorization, cache-control, CORS
  proxy: reverse proxy, load balancer, API gateway, forwarding

@messaging — async message queues and event brokers
  queues: enqueue, dequeue, dead letter, retry, priority
  pubsub: topics, subscriptions, fan-out, filtering
  brokers: RabbitMQ, Kafka, SQS, NATS, Redis Streams
  patterns: saga, outbox, exactly-once, at-least-once

@email — sending, receiving, and processing email
  sending: SMTP, transactional, templates, attachments
  receiving: IMAP, POP3, webhooks, inbound parsing
  deliverability: SPF, DKIM, DMARC, bounce handling
  templates: HTML email, mjml, plain text, personalization

@notifications — push, in-app, and multi-channel alerts
  push: mobile push, web push, FCM, APNs, tokens
  in_app: notification center, badges, toasts, bell icon
  channels: SMS, Slack, webhook, dispatch routing
  preferences: opt-in, frequency, digest, unsubscribe

@streaming — real-time data streams and SSE
  sse: server-sent events, event source, reconnection
  data_streams: Kafka Streams, Kinesis, Flink, windowing
  backpressure: flow control, buffering, rate adaptation
  replay: event replay, offset management, checkpoint

@serialization — data format encoding and decoding
  json: marshal, unmarshal, custom encoders, streaming
  binary: protobuf, msgpack, avro, CBOR, flatbuffers
  xml: parsing, generation, XPath, XSLT, namespaces
  schema_validation: JSON Schema, OpenAPI spec, Avro schema

@rate_limiting — throttling, quotas, and traffic shaping
  algorithms: token bucket, sliding window, leaky bucket, fixed window
  enforcement: middleware, API gateway, per-user, per-IP
  headers: X-RateLimit, Retry-After, quota reset
  distributed: Redis-based, shared state, cluster coordination

@webhooks — outgoing and incoming webhook management
  outgoing: delivery, retry, signing, payload construction
  incoming: verification, parsing, idempotency keys
  management: registration, rotation, failure alerting
  security: signature verification, HMAC, replay prevention
```

**Count: 12 domains**

---

## Focus Area 3: Data & Storage (~14 domains)

```
@database — relational database operations
  queries: SELECT, INSERT, UPDATE, DELETE, joins, subqueries
  schema: tables, columns, constraints, indexes, foreign keys
  migrations: up/down, versioning, rollback, seed data
  transactions: BEGIN, COMMIT, ROLLBACK, isolation levels, deadlocks

@orm — object-relational mapping and model layer
  models: definition, fields, types, defaults, validators
  relations: belongs_to, has_many, many_to_many, polymorphic
  querying: scopes, eager loading, lazy loading, raw queries
  lifecycle: callbacks, hooks, before_save, after_create

@nosql — document stores, key-value, wide-column
  document: MongoDB, CouchDB, collections, aggregation pipeline
  key_value: Redis, DynamoDB, get/set, TTL, atomic operations
  wide_column: Cassandra, BigTable, column families, partitioning
  graph: Neo4j, vertices, edges, traversal, Cypher

@caching — application and data caching layers
  strategies: read-through, write-behind, cache-aside, refresh-ahead
  invalidation: TTL, explicit purge, tag-based, event-driven
  layers: L1 memory, L2 Redis, CDN edge, browser cache
  patterns: memoization, HTTP caching, query result cache

@search_engine — full-text search and information retrieval
  indexing: analyzers, tokenizers, mappings, reindex
  querying: full-text, fuzzy, phrase, bool, aggregations
  ranking: scoring, boosting, relevance, custom scoring
  infrastructure: Elasticsearch, Solr, Meilisearch, shards, replicas

@file_storage — file upload, download, and management
  upload: multipart, chunked, presigned URLs, progress
  cloud: S3, GCS, Azure Blob, bucket policies
  processing: resize, transcode, thumbnail, virus scan
  serving: CDN, signed URLs, streaming, range requests

@data_pipeline — ETL, batch processing, data movement
  extraction: connectors, CDC, polling, webhooks
  transformation: mapping, filtering, enrichment, deduplication
  loading: bulk insert, upsert, merge, partitioning
  orchestration: Airflow, Dagster, Prefect, DAGs, scheduling

@analytics — metrics, aggregation, and data analysis
  events: tracking, event schema, properties, user identification
  aggregation: rollup, OLAP, cubes, materialized views
  reporting: dashboards, charts, export, scheduled reports
  tools: BigQuery, Redshift, Snowflake, ClickHouse

@data_modeling — schema design and entity relationships
  entities: attributes, relationships, cardinality, constraints
  normalization: 1NF, 2NF, 3NF, denormalization tradeoffs
  patterns: soft delete, polymorphism, EAV, audit trail
  versioning: schema evolution, backward compatibility, migrations

@time_series — temporal data storage and querying
  ingestion: metrics, events, timestamps, resolution
  querying: range queries, downsampling, retention policies
  databases: InfluxDB, TimescaleDB, Prometheus, VictoriaMetrics
  patterns: rollup, compaction, partitioning by time

@blob_processing — binary data and media processing
  images: resize, crop, format conversion, EXIF, optimization
  video: transcode, thumbnail, HLS, DASH, ffmpeg
  audio: conversion, waveform, transcription, streaming
  documents: PDF generation, parsing, OCR, watermark

@geospatial — location data, maps, and spatial queries
  coordinates: latitude, longitude, GeoJSON, WKT
  queries: within, intersects, nearest, distance, bounding box
  services: geocoding, routing, tile servers, reverse geocode
  indexing: R-tree, geohash, H3, spatial index

@replication — data sync, replication, and consistency
  strategies: master-slave, multi-master, eventual, strong
  conflict: resolution, CRDT, vector clocks, last-write-wins
  sync: bidirectional, change feed, log-based, snapshot
  failover: promotion, health check, automatic, manual

@backup — data backup, recovery, and disaster planning
  strategies: full, incremental, differential, continuous
  storage: offsite, encrypted, retention policy, tiering
  recovery: point-in-time, restore, validation, RTO/RPO
  automation: scheduled, triggered, verification, alerting
```

**Count: 14 domains**

---

## Focus Area 4: Frontend Core (~10 domains)

```
@components — UI component architecture and lifecycle
  rendering: mount, unmount, update, virtual DOM, reconciliation
  composition: props, children, slots, higher-order, compound
  state: local state, derived state, refs, context
  lifecycle: effects, cleanup, memo, lazy initialization

@state_management — client-side state architecture
  stores: global store, slices, modules, atoms
  updates: dispatch, actions, reducers, mutations, signals
  selectors: derived state, memoized selectors, computed
  middleware: thunks, sagas, effects, devtools

@routing — client-side navigation and URL management
  routes: path matching, params, query strings, wildcards
  navigation: push, replace, redirect, guards, middleware
  patterns: nested routes, dynamic routes, code splitting
  history: browser history, hash routing, memory routing

@forms — form state, validation, and submission
  fields: input, select, checkbox, radio, textarea, file
  validation: rules, schema validation, async validation, errors
  state: dirty, touched, pristine, submitting, submitted
  patterns: controlled, uncontrolled, field arrays, wizard/stepper

@styling — CSS architecture and theming
  methodology: BEM, CSS modules, CSS-in-JS, utility-first
  theming: tokens, variables, dark mode, color schemes
  responsive: breakpoints, media queries, container queries
  preprocessors: Sass, Less, PostCSS, Tailwind, Styled

@animation — motion, transitions, and visual effects
  transitions: enter, exit, opacity, transform, duration
  keyframes: sequences, spring physics, gesture-driven
  scroll: parallax, scroll-driven, intersection observer
  libraries: Framer Motion, GSAP, CSS transitions, Lottie

@accessibility — a11y compliance and inclusive design
  semantics: ARIA roles, landmarks, labels, live regions
  interaction: keyboard navigation, focus management, tab order
  visual: contrast, font size, color blindness, reduced motion
  testing: screen reader, axe, lighthouse, WCAG levels

@data_fetching — client-server data synchronization
  requests: fetch, axios, interceptors, abort, timeout
  caching: SWR, React Query, cache invalidation, optimistic
  pagination: cursor, offset, infinite scroll, virtual
  real_time: polling, SSE, WebSocket subscriptions

@browser — browser APIs and Web Platform
  storage: localStorage, sessionStorage, IndexedDB, cookies
  workers: Web Worker, Service Worker, SharedWorker
  apis: Clipboard, Notifications, Geolocation, MediaStream
  events: DOM events, custom events, event delegation

@dom — DOM manipulation and rendering strategies
  manipulation: createElement, querySelector, mutation observer
  rendering: SSR, CSR, SSG, ISR, hydration, streaming
  performance: virtual scrolling, lazy loading, code splitting
  templates: template literals, JSX, compiled templates
```

**Count: 10 domains**

---

## Focus Area 5: Mobile (~8 domains)

```
@mobile_navigation — screen management and navigation patterns
  stacks: push, pop, modal, nested navigators
  tabs: bottom tabs, top tabs, tab badges, lazy tabs
  deep_linking: URL schemes, universal links, deferred
  transitions: screen transitions, shared element, gestures

@mobile_ui — native UI patterns and platform conventions
  gestures: tap, swipe, pinch, long press, pan
  lists: flat list, section list, pull to refresh, infinite scroll
  layout: safe area, keyboard avoiding, platform specific
  feedback: haptic, vibration, toast, snackbar, loading

@push_notifications — remote and local notification systems
  remote: FCM, APNs, token registration, payload, silent push
  local: scheduled, recurring, actions, categories
  handling: foreground, background, tap response, deep link
  management: channels, preferences, badge count, grouping

@app_lifecycle — application state and lifecycle management
  states: foreground, background, suspended, terminated
  startup: cold start, warm start, splash screen, initialization
  memory: warnings, cleanup, caching strategy, leak detection
  updates: app update, force update, feature flags, rollback

@mobile_storage — on-device data persistence
  secure: keychain, keystore, encrypted preferences
  local: SQLite, Realm, Core Data, Room, MMKV
  sync: offline first, conflict resolution, queue, background sync
  files: documents, cache directory, shared containers

@mobile_platform — platform-specific APIs and integrations
  camera: capture, gallery, permissions, barcode, AR
  location: GPS, geofencing, background location, accuracy
  biometrics: Face ID, Touch ID, fingerprint, device lock
  contacts: address book, picker, permissions, sync

@cross_platform — shared code across iOS and Android
  bridge: native modules, platform channels, FFI
  rendering: native views, custom renderers, platform detect
  tooling: React Native, Flutter, Kotlin Multiplatform, Capacitor
  sharing: shared logic, platform abstraction, expect/actual

@wearable — smartwatch and companion device development
  watch: complications, glances, tiles, watch faces
  health: heart rate, steps, workout, HealthKit, Google Fit
  connectivity: phone-watch sync, standalone, Bluetooth
  constraints: small screen, battery, limited input
```

**Count: 8 domains**

---

## Focus Area 6: Security (~8 domains)

```
@security_scanning — vulnerability detection and code analysis
  sast: static analysis, code scanning, pattern matching
  dast: dynamic testing, fuzzing, penetration testing
  dependencies: CVE scanning, SCA, license compliance, advisories
  secrets: secret scanning, pre-commit hooks, rotation detection

@input_validation — sanitization and injection prevention
  sql_injection: parameterized queries, prepared statements, escaping
  xss: output encoding, CSP, sanitization, DOMPurify
  command_injection: shell escaping, allowlists, sandboxing
  path_traversal: path normalization, chroot, allowlisted paths

@network_security — transport and network-level protection
  tls: certificates, pinning, mutual TLS, cipher suites
  cors: origins, methods, headers, credentials, preflight
  csrf: tokens, SameSite cookies, double submit, origin check
  firewall: WAF, IP filtering, geo-blocking, DDoS mitigation

@compliance — regulatory and audit requirements
  gdpr: consent, data subject rights, DPA, processing records
  hipaa: PHI, BAA, encryption requirements, audit logging
  pci: cardholder data, tokenization, scope reduction
  audit: audit trail, immutable logs, retention, access logs

@secure_coding — defensive programming practices
  boundaries: trust boundaries, input/output validation
  least_privilege: minimal permissions, principle of least authority
  fail_safe: secure defaults, deny by default, safe error handling
  supply_chain: dependency pinning, lockfiles, reproducible builds

@identity_security — credential and identity protection
  password_policy: complexity, breach detection, rotation
  credential_storage: hashing, salting, peppering, timing attacks
  brute_force: lockout, CAPTCHA, progressive delay, IP blocking
  session_security: fixation, hijacking, invalidation, binding

@data_protection — data at rest and in transit
  encryption_at_rest: disk encryption, field-level, database TDE
  encryption_in_transit: TLS, certificate management, HSTS
  masking: PII masking, tokenization, anonymization, pseudonymization
  classification: data levels, sensitivity labels, handling rules

@threat_modeling — security architecture and risk assessment
  patterns: STRIDE, DREAD, attack trees, kill chains
  assets: crown jewels, data flow, trust boundaries, entry points
  mitigations: controls, countermeasures, risk acceptance
  review: security review, threat assessment, red team
```

**Count: 8 domains**

---

## Focus Area 7: Testing & Quality (~7 domains)

```
@unit_testing — isolated component and function testing
  assertions: expect, assert, matchers, custom assertions
  structure: describe, it, test, suite, spec, given/when/then
  isolation: mock, stub, spy, fake, double, dependency injection
  fixtures: factory, builder, seed data, shared context

@integration_testing — multi-component and service testing
  database: test database, migrations, transactions, cleanup
  api: request testing, response validation, contract testing
  services: service mocks, test containers, test servers
  setup: before/after, fixtures, database seeding, state reset

@e2e_testing — browser and UI automation testing
  automation: Selenium, Playwright, Cypress, Puppeteer
  interactions: click, type, select, wait, navigate, screenshot
  selectors: CSS, XPath, test ID, accessibility selectors
  patterns: page object, screenplay, visual regression

@performance_testing — load, stress, and benchmark testing
  load: concurrent users, throughput, response time, ramp-up
  stress: breaking point, saturation, recovery, soak
  benchmarks: micro-benchmark, flamegraph, profiling, baselines
  tools: k6, Gatling, JMeter, Artillery, wrk

@test_infrastructure — CI testing and test management
  coverage: line, branch, function, threshold, reporting
  ci: pipeline, parallel, matrix, retry, artifacts
  management: test plans, test suites, flaky detection, quarantine
  mocking_services: WireMock, mock server, service virtualization

@code_quality — linting, formatting, and static checks
  linting: ESLint, Pylint, golangci-lint, rules, plugins
  formatting: Prettier, gofmt, black, editorconfig
  type_checking: TypeScript, mypy, Flow, type inference
  analysis: complexity, duplication, dependency analysis, tech debt

@documentation — code docs, API docs, and knowledge base
  code_docs: JSDoc, GoDoc, Sphinx, docstrings, annotations
  api_docs: OpenAPI, Swagger, Redoc, API reference
  guides: README, getting started, tutorials, examples
  generation: auto-generated, changelog, release notes
```

**Count: 7 domains**

---

## Focus Area 8: Infrastructure & DevOps (~14 domains)

```
@containers — container build, runtime, and orchestration
  docker: Dockerfile, build, layers, multi-stage, compose
  runtime: entrypoint, volumes, networks, health checks
  registry: push, pull, tags, scanning, signing
  optimization: layer caching, slim images, distroless, buildpacks

@kubernetes — container orchestration and cluster management
  workloads: deployment, statefulset, daemonset, job, cronjob
  networking: service, ingress, network policy, DNS, load balancer
  config: configmap, secret, environment, volume mounts
  operations: scaling, rolling update, rollback, HPA, VPA

@ci_cd — continuous integration and delivery pipelines
  pipelines: stages, jobs, steps, triggers, caching
  builds: compile, test, lint, artifacts, docker build
  deployment: deploy stages, approvals, rollback, canary, blue-green
  platforms: GitHub Actions, GitLab CI, Jenkins, CircleCI

@cloud — cloud provider services and IaC
  compute: EC2, Cloud Run, Lambda, VM, serverless
  networking: VPC, subnets, security groups, load balancers
  storage: S3, GCS, EBS, managed disks
  iac: Terraform, Pulumi, CloudFormation, CDK

@monitoring — system and application observability
  metrics: Prometheus, Grafana, counters, gauges, histograms
  alerting: thresholds, PagerDuty, OpsGenie, escalation, silence
  dashboards: panels, queries, variables, annotations
  health: health checks, readiness, liveness, startup probes

@logging — structured logging and log management
  structured: JSON logs, log levels, context, correlation ID
  collection: Fluentd, Logstash, Vector, log rotation
  storage: Elasticsearch, Loki, CloudWatch, retention
  analysis: search, filter, aggregation, log patterns

@tracing — distributed tracing and request flow
  instrumentation: spans, traces, baggage, context propagation
  collection: OpenTelemetry, Jaeger, Zipkin, exporters
  analysis: trace visualization, latency breakdown, error traces
  correlation: trace ID, span ID, parent span, sampling

@dns — domain management and name resolution
  records: A, AAAA, CNAME, MX, TXT, SRV, NS
  management: registrar, nameservers, TTL, propagation
  routing: weighted, latency-based, geolocation, failover
  security: DNSSEC, CAA, certificate transparency

@networking — network configuration and management
  protocols: TCP, UDP, HTTP/2, HTTP/3, QUIC
  configuration: ports, interfaces, routing tables, NAT
  vpn: WireGuard, IPSec, tunneling, mesh networking
  service_mesh: Istio, Linkerd, sidecar, mTLS, traffic management

@serverless — function-as-a-service and event-driven compute
  functions: Lambda, Cloud Functions, Azure Functions, Deno Deploy
  triggers: HTTP, queue, schedule, S3 event, DynamoDB stream
  patterns: cold start, warm pool, concurrency, timeout
  frameworks: Serverless Framework, SAM, SST, Architect

@infrastructure_security — infrastructure hardening and access
  iam: policies, roles, service accounts, assume role
  network: security groups, NACLs, private subnets, bastion
  secrets: Vault, Secrets Manager, Parameter Store, SOPS
  compliance: CIS benchmarks, SOC2, audit logging, drift detection

@release — versioning, releases, and feature management
  versioning: semver, changelog, release notes, tagging
  feature_flags: toggles, gradual rollout, A/B testing, kill switch
  rollout: canary, blue-green, percentage, ring deployment
  rollback: automatic, manual, version pinning, hotfix

@package_management — dependency and artifact management
  registries: npm, PyPI, Maven, crates.io, Go modules
  lockfiles: lockfile, pinning, resolution, deduplication
  publishing: publish, version bump, prepublish, access
  security: audit, advisory, vulnerability, override

@git — version control operations and workflows
  branching: branch, merge, rebase, cherry-pick, squash
  workflow: PR, code review, approval, CI checks
  operations: stash, reset, bisect, blame, reflog
  hooks: pre-commit, pre-push, commit-msg, husky
```

**Count: 14 domains**

---

## Focus Area 9: Architecture Patterns (~12 domains)

```
@event_driven — event sourcing, CQRS, and event architecture
  sourcing: event store, aggregate, projection, replay
  cqrs: command, query, read model, write model, eventual
  bus: event bus, command bus, dispatch, handler registration
  patterns: saga, process manager, compensating transaction

@microservices — distributed service architecture
  decomposition: bounded context, service boundary, domain
  communication: service-to-service, sync, async, choreography
  discovery: service registry, DNS, health, load balancing
  resilience: circuit breaker, bulkhead, retry, timeout, fallback

@dependency_injection — IoC containers and service wiring
  containers: register, resolve, lifetime, scope
  patterns: constructor injection, method injection, factory
  lifecycle: singleton, transient, scoped, lazy initialization
  frameworks: Spring, Guice, Wire, dig, fx

@middleware — request/response pipeline and interceptors
  http: request middleware, response middleware, chain, next
  error: error handling middleware, recovery, panic handler
  common: logging, auth, CORS, compression, rate limiting
  patterns: decorator, chain of responsibility, pipeline

@plugin_system — extensibility and plugin architecture
  loading: dynamic loading, plugin discovery, registration
  lifecycle: init, start, stop, health, configuration
  api: plugin API, hooks, extension points, contracts
  isolation: sandboxing, permissions, versioning, compatibility

@configuration — application config and environment management
  sources: env vars, config files, CLI flags, remote config
  parsing: YAML, TOML, JSON, dotenv, hierarchical
  validation: required, defaults, type coercion, constraints
  patterns: 12-factor, config server, hot reload, feature config

@error_handling — error management and recovery strategies
  types: domain errors, infrastructure errors, validation errors
  propagation: wrapping, context, stack traces, error chains
  recovery: retry, fallback, circuit breaker, graceful degradation
  reporting: error tracking, Sentry, Bugsnag, grouping

@concurrency — parallel execution and synchronization
  primitives: mutex, semaphore, channel, atomic, wait group
  patterns: worker pool, fan-out/fan-in, pipeline, producer-consumer
  async: promises, futures, async/await, coroutines, goroutines
  safety: race condition, deadlock, livelock, thread safety

@scheduling — task scheduling and background processing
  cron: cron expressions, recurring, timezone, overlap handling
  queues: job queue, priority, delay, retry, dead letter
  workers: background worker, pool, concurrency limit
  frameworks: Sidekiq, Celery, Bull, Hangfire, cron

@state_machine — workflow and state transition management
  definition: states, transitions, events, guards, actions
  patterns: finite state, hierarchical, parallel states
  persistence: state storage, history, rehydration
  tools: XState, AASM, statechart, workflow engine

@cqrs — command-query separation at architectural level
  commands: command objects, handlers, validation, authorization
  queries: query objects, projections, read optimization
  separation: write model, read model, eventual consistency
  sync: projection rebuilding, catch-up, snapshot
```

Hmm, CQRS overlaps with @event_driven. Removing it.

```
@clean_architecture — layered architecture and domain boundaries
  layers: domain, application, infrastructure, presentation
  boundaries: ports, adapters, hexagonal, onion
  dependencies: dependency rule, inversion, abstraction
  patterns: use case, entity, value object, aggregate
```

**Count: 11 domains (removed @cqrs duplicate)**

---

## Focus Area 10: Systems & Low-Level (~8 domains)

```
@memory — memory management and optimization
  allocation: heap, stack, arena, pool, slab
  garbage_collection: GC tuning, generations, pause, finalizers
  leaks: detection, profiling, weak references, cycle breaking
  optimization: cache locality, alignment, zero-copy, mmap

@process — process and thread management
  lifecycle: spawn, fork, exec, wait, signal, exit
  ipc: pipe, socket, shared memory, message queue, RPC
  signals: SIGTERM, SIGINT, SIGHUP, signal handling, graceful
  management: PID, daemon, supervisor, watchdog, restart

@filesystem — file and directory operations
  operations: read, write, stat, rename, delete, copy
  watching: inotify, fsnotify, polling, change detection
  permissions: chmod, chown, ACL, umask, capabilities
  patterns: temp files, atomic write, lock file, directory walk

@io — input/output streams and buffering
  streams: reader, writer, buffered, piped, tee
  async: non-blocking, epoll, kqueue, io_uring, IOCP
  encoding: UTF-8, binary, base64, hex, endianness
  compression: gzip, zstd, brotli, snappy, deflate

@cli — command-line interface design and parsing
  parsing: flags, arguments, subcommands, completion
  output: colors, tables, progress bars, spinners
  interaction: prompts, confirmation, selection, password input
  frameworks: cobra, click, argparse, clap, commander

@regex — pattern matching and text processing
  syntax: groups, lookahead, lookbehind, quantifiers, alternation
  engines: PCRE, RE2, backtracking, NFA, DFA
  usage: validation, extraction, substitution, splitting
  performance: compilation, caching, catastrophic backtracking

@date_time — temporal operations and timezone handling
  parsing: format strings, ISO 8601, RFC 3339, natural language
  arithmetic: duration, interval, add, subtract, difference
  timezone: IANA, UTC, conversion, DST, offset
  formatting: locale-specific, relative, calendar, epoch

@math — numerical computation and algorithms
  arithmetic: precision, overflow, rounding, big numbers
  statistics: mean, median, percentile, standard deviation
  algorithms: sort, search, graph, hash, bloom filter
  random: PRNG, CSPRNG, UUID, snowflake, distribution
```

**Count: 8 domains**

---

## Focus Area 11: Web Platform (~6 domains)

```
@seo — search engine optimization and web discoverability
  metadata: title, description, OG tags, Twitter cards
  structured: JSON-LD, Schema.org, rich snippets, breadcrumbs
  technical: sitemap, robots.txt, canonical, hreflang
  performance: Core Web Vitals, LCP, FID, CLS

@pwa — progressive web app features
  service_worker: caching strategies, offline, background sync
  manifest: install prompt, icons, theme, display mode
  push: web push, subscription, notification, permission
  storage: cache API, IndexedDB, storage manager, quota

@bundling — asset compilation and module bundling
  bundlers: webpack, Vite, esbuild, Rollup, Parcel
  optimization: tree shaking, code splitting, minification, chunks
  loaders: CSS, images, fonts, SVG, raw imports
  config: entry points, output, aliases, resolve, plugins

@content_management — CMS and content delivery
  headless: API-driven, structured content, content types
  markdown: parsing, rendering, frontmatter, MDX
  media: image optimization, responsive images, lazy loading
  static: SSG, JAMstack, build-time rendering, ISR

@performance — web and application performance optimization
  metrics: TTFB, FCP, LCP, INP, CLS, bundle size
  optimization: lazy loading, preloading, prefetching, caching
  profiling: Chrome DevTools, flamegraph, memory profiling
  budgets: performance budgets, lighthouse, monitoring

@web_security — browser security features and headers
  csp: Content-Security-Policy, nonce, hash, report-uri
  headers: X-Frame-Options, X-Content-Type, Referrer-Policy
  sandbox: iframe sandbox, permissions policy, COEP, COOP
  storage: secure cookies, storage partitioning, SameSite
```

**Count: 6 domains**

---

## Focus Area 12: Machine Learning & Data Science (~8 domains)

```
@ml_training — model training and experimentation
  training: epochs, batch size, learning rate, loss, optimizer
  evaluation: accuracy, precision, recall, F1, confusion matrix
  tuning: hyperparameters, grid search, cross-validation
  tracking: experiment tracking, MLflow, W&B, metrics logging

@ml_inference — model serving and prediction
  serving: model server, REST endpoint, batch prediction
  optimization: quantization, pruning, ONNX, TensorRT
  pipeline: preprocessing, postprocessing, feature extraction
  monitoring: drift detection, accuracy tracking, A/B testing

@ml_data — dataset management and feature engineering
  datasets: loading, splitting, augmentation, sampling
  features: extraction, selection, transformation, encoding
  labeling: annotation, active learning, weak supervision
  versioning: DVC, data lineage, snapshots, reproducibility

@nlp — natural language processing
  tokenization: word, subword, BPE, sentence segmentation
  models: embeddings, transformers, BERT, GPT, fine-tuning
  tasks: classification, NER, sentiment, summarization, QA
  processing: cleaning, normalization, stemming, lemmatization

@computer_vision — image and video analysis
  detection: object detection, YOLO, bounding boxes, segmentation
  classification: image classification, transfer learning, CNN
  preprocessing: resize, normalize, augmentation, color space
  video: frame extraction, tracking, optical flow, action recognition

@llm — large language model integration
  prompting: prompt engineering, few-shot, chain-of-thought, RAG
  apis: OpenAI, Anthropic, chat completion, streaming
  embedding: vector embedding, similarity search, vector DB
  agents: tool use, function calling, chains, memory

@data_visualization — charts, graphs, and visual analytics
  charts: bar, line, scatter, histogram, heatmap
  libraries: D3, Chart.js, Plotly, matplotlib, Recharts
  interactive: zoom, pan, tooltip, brush, selection
  export: PNG, SVG, PDF, embedded, responsive

@notebook — interactive computing and exploration
  jupyter: cells, kernel, magic commands, widgets
  workflow: exploration, prototyping, presentation, sharing
  integration: pandas, numpy, scipy, SQL magic
  output: rich display, HTML, plots, tables, LaTeX
```

**Count: 8 domains**

---

## Focus Area 13: Developer Workflow (~6 domains)

```
@debugging — troubleshooting and diagnostic tools
  breakpoints: set, conditional, hit count, logpoint
  inspection: watch, evaluate, call stack, scope variables
  tools: debugger, REPL, Chrome DevTools, lldb, gdb
  techniques: bisect, trace, print debugging, core dump

@profiling — performance analysis and optimization
  cpu: flamegraph, hot path, sampling, instrumentation
  memory: heap snapshot, allocation tracking, leak detection
  network: waterfall, timing, request/response analysis
  tools: pprof, py-spy, perf, Chrome DevTools, Instruments

@logging_dev — development-time logging and diagnostics
  levels: debug, info, warn, error, trace, fatal
  context: request ID, user ID, correlation, breadcrumbs
  output: console, file, remote, structured
  tools: debug libraries, pretty print, color output

@build_system — compilation, linking, and build tooling
  compilers: gcc, clang, go build, javac, rustc, tsc
  build_tools: Make, CMake, Gradle, Bazel, Meson
  artifacts: binary, library, bundle, container image
  optimization: incremental, parallel, caching, distributed

@editor_tooling — IDE integration and developer experience
  lsp: language server, completion, diagnostics, hover
  extensions: plugins, formatters, linters, debugger adapters
  config: workspace settings, tasks, launch config, snippets
  ai: copilot, code completion, inline suggestions, chat

@project_setup — project initialization and scaffolding
  scaffolding: create-react-app, cookiecutter, yeoman, init
  structure: directory layout, entry points, config files
  standards: gitignore, editorconfig, license, contributing
  monorepo: workspaces, lerna, nx, turborepo, changesets
```

**Count: 6 domains**

---

## Focus Area 14: Domain-Specific (~10 domains)

```
@ecommerce — online commerce and purchasing
  catalog: products, categories, variants, pricing, inventory
  cart: add, remove, quantity, subtotal, discount
  checkout: payment flow, shipping, tax, order creation
  orders: status, fulfillment, refund, cancellation, tracking

@payment — payment processing and financial transactions
  processing: charge, capture, refund, void, authorization
  providers: Stripe, PayPal, Braintree, Adyen, Square
  methods: credit card, bank transfer, wallet, cryptocurrency
  compliance: PCI DSS, tokenization, 3D Secure, fraud detection

@subscription — recurring billing and subscription management
  billing: plans, pricing tiers, proration, invoicing
  lifecycle: trial, active, paused, canceled, expired
  management: upgrade, downgrade, add-ons, entitlements
  metering: usage-based, per-seat, credits, overage

@cms_content — content creation and editorial workflows
  content: pages, posts, blocks, rich text, media
  workflow: draft, review, publish, schedule, archive
  taxonomy: categories, tags, collections, hierarchies
  localization: multilingual, translation, locale, fallback

@social — social features and community
  feed: timeline, activity stream, aggregation, ranking
  relationships: follow, friend, block, mute, connections
  interactions: like, comment, share, reaction, mention
  moderation: report, review, ban, content filter, spam

@gaming — game development and game systems
  engine: game loop, scene, entity, component, system
  physics: collision, rigid body, ray cast, force, gravity
  rendering: sprite, mesh, shader, camera, lighting
  input: keyboard, gamepad, touch, gesture, VR controller

@iot — Internet of Things and embedded communication
  protocols: MQTT, CoAP, BLE, Zigbee, LoRa
  devices: sensor, actuator, gateway, edge, firmware
  management: provisioning, OTA updates, fleet, telemetry
  data: time series, aggregation, thresholds, alerting

@blockchain — distributed ledger and smart contracts
  contracts: Solidity, deploy, interact, ABI, events
  transactions: signing, gas, nonce, receipt, confirmation
  wallets: address, key management, HD wallet, recovery
  defi: token, swap, liquidity, staking, yield

@audio_video — media playback and communication
  playback: player, controls, playlist, seek, buffering
  communication: WebRTC, SDP, ICE, STUN, TURN, peer
  recording: capture, encoding, format, quality, duration
  streaming: HLS, DASH, adaptive, low latency, CDN

@maps — mapping, routing, and spatial visualization
  display: tile layer, marker, popup, overlay, cluster
  interaction: pan, zoom, draw, measure, geocode
  routing: directions, waypoints, distance, duration, traffic
  providers: Mapbox, Google Maps, Leaflet, OpenStreetMap
```

**Count: 10 domains**

---

## Focus Area 15: Language-Specific Patterns (~6 domains)

```
@type_system — type definitions, generics, and type safety
  generics: type parameters, constraints, variance, inference
  types: interface, struct, enum, union, alias, literal
  advanced: conditional types, mapped types, utility types
  validation: runtime validation, type guards, narrowing, Zod

@functional — functional programming patterns
  core: pure functions, immutability, composition, currying
  types: Maybe/Option, Either/Result, IO, Task
  collections: map, filter, reduce, flatMap, fold
  patterns: monad, functor, applicative, lens, transducer

@oop — object-oriented design patterns and principles
  patterns: factory, observer, strategy, adapter, decorator
  principles: SOLID, DRY, composition over inheritance
  structure: class, interface, abstract, mixin, trait
  relationships: inheritance, composition, delegation, association

@reactive — reactive programming and streams
  observables: subscribe, pipe, operators, subjects
  operators: map, filter, merge, switchMap, debounce
  patterns: reactive forms, event streams, state streams
  libraries: RxJS, Reactor, RxJava, Combine, Flow

@metaprogramming — code generation and reflection
  reflection: inspect, attributes, annotations, decorators
  codegen: templates, macros, AST manipulation, source gen
  dynamic: eval, dynamic dispatch, proxy, interceptor
  compile_time: const eval, build scripts, proc macros

@interop — cross-language integration and FFI
  ffi: C bindings, ABI, calling conventions, memory layout
  wasm: WebAssembly, WASI, compile targets, host functions
  embedding: scripting, Lua, Python, JavaScript engine
  serialization: cross-language serialization, IDL, Thrift
```

**Count: 6 domains**

---

## Running Total

| Focus Area | Domains |
|---|---|
| Authentication & Identity | 6 |
| API & Communication | 12 |
| Data & Storage | 14 |
| Frontend Core | 10 |
| Mobile | 8 |
| Security | 8 |
| Testing & Quality | 7 |
| Infrastructure & DevOps | 14 |
| Architecture Patterns | 11 |
| Systems & Low-Level | 8 |
| Web Platform | 6 |
| Machine Learning & Data Science | 8 |
| Developer Workflow | 6 |
| Domain-Specific | 10 |
| Language-Specific Patterns | 6 |
| **TOTAL** | **134** |

---

## Gap Analysis

Areas that might be underrepresented:
- **Database variants**: We have @database (relational) and @nosql but no dedicated @sqlite, @postgres, @mysql — should these be terms under @database or separate?
- **Specific frameworks**: No @react, @vue, @angular, @django, @rails, @spring — should popular frameworks be domains or terms under broader categories?
- **Cloud-specific**: @cloud is broad — should @aws, @gcp, @azure be separate?
- **Internationalization**: Currently a term under @accessibility — deserves its own domain?
- **Multi-tenancy**: No dedicated domain for tenant isolation, shared infrastructure
- **Workflow/BPM**: Beyond state machines — orchestration, approval flows
- **PDF/document generation**: Under @blob_processing but may deserve its own domain

To reach ~200, we could expand into:
- Framework-specific domains (~20: React, Vue, Angular, Next, Nuxt, Django, Rails, Spring, Laravel, Express, FastAPI, Flask, .NET, Svelte, Remix, Astro, Deno, Bun, NestJS, Gin)
- Cloud provider domains (~6: AWS, GCP, Azure, Cloudflare, Vercel, Netlify)
- Additional specialized (~10-15: i18n, multi_tenancy, workflow, pdf, spreadsheet, ...)

Should we expand framework-specific domains, or keep those as terms under broader domains?
