<div align="center">

# 🐦 Sparrow

![Sparrow](logo.png)
<br>
**A Modern CQRS Framework for Go**

[![Go Version](https://img.shields.io/badge/Go-1.25-00ADD8?style=flat-square&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green?style=flat-square)](LICENSE)
[![Build](https://img.shields.io/badge/Build-Passing-brightgreen?style=flat-square)]()

*Event-driven · Domain Modeling · Multi-storage · Service Autonomy · Production Ready*

</div>

<div align="center">

[English](./README.md) | [中文](./README.zh.md) | [详细文档](http://sparrow.rayainfo.cn/)

</div>


## Features

### Usage Side

- **Zero Boilerplate** — Aggregate root, repository, event publishing 3 in 1，inherit and use, no infrastructure repetition
- **Interface as Contract** — Repository, event bus, event store all interface-oriented, mock testing effortless
- **Progressive Adoption** — Start with in-memory implementation, replace DI in production, zero business code changes
- **Unified Error Semantics** — Global error wrapping specification, cross-layer context preservation, observability out-of-the-box
- **Configuration-driven Startup** — Viper multi-source configuration (file / env / remote), one `App` lifecycle manages all services

### Technical Side

- **Clean Architecture Layers** — Strict dependency inversion, domain layer zero infrastructure awareness, architecture corruption blocked at source
- **Generic Repository DSL** — Go generics powered, type-safe CRUD / pagination / soft delete, one interface for multiple databases
- **Event Bus Multi-backend** — Memory / NATS JetStream / RabbitMQ / Redis, unified interface, runtime switchable
- **NATS JetStream Native Integration** — Persistent event streams, consumer groups, replay capabilities, native Event Sourcing support
- **Saga Distributed Transactions** — Orchestrated Saga coordinator, clear compensation transaction chains, cross-service consistency guaranteed
- **Testcontainers Integration Testing** — PostgreSQL, Redis real container testing, no more mock database false positives

### Microservice Autonomy

> **One service, one database** is no longer a奢望

The ultimate goal of microservice architecture is **complete autonomy** — each service has independent data storage, no sharing, no coupling. Sparrow has two built-in aces to make this goal achievable:

|                             | NATS                                                 | BadgerDB                                         |
| --------------------------- | ---------------------------------------------------- | ------------------------------------------------ |
| **Position**                | Lightweight high-performance event stream            | Embedded billion-level KV database               |
| **Binary Size**             | < 20 MB                                              | No independent process, starts with service      |
| **Throughput**              | Millions of messages/sec                             | Millions of ops/sec (SSD)                        |
| **Deployment Dependencies** | Single executable, no operation and maintenance      | Zero external dependencies, data follows service |
| **Role in Sparrow**         | Inter-service event communication and event sourcing | Service private read model / state storage       |

```
┌──────────────┐     NATS JetStream     ┌──────────────┐
│  Order Service │ ──── OrderPlaced ────► │  Inventory Service │
│  BadgerDB    │                        │  BadgerDB    │
│  (Private Storage) │ ◄─── StockReserved ─── │  (Private Storage) │
└──────────────┘                        └──────────────┘
     ↑ No shared database, services communicate only through events, fully decoupled and autonomous
```

This means: each microservice can **deploy independently, scale independently, upgrade independently** without other services being aware.

---

## Why Choose Sparrow?

Traditional microservice development often falls into three dilemmas:

- **Business and data tight coupling** — REST direct database connection, audit and replay impossible
- **Cross-service transaction nightmare** — Synchronous call chains, consistency problems unsolvable
- **Infrastructure reinventing the wheel** — Repository layer, error handling, event carrying each for their own

Sparrow solves these problems at once with a complete toolchain of DDD + CQRS + Event Sourcing.

---

## Core Capabilities

### Event-driven Architecture

Unified event bus abstraction, supports multiple backends, switch on demand, no business code changes:

```go
// Subscribe to domain events
bus.Sub("OrderPlaced", func(ctx context.Context, evt eventbus.Event) error {
    // Handle order creation event
    return nil
})

// Publish event
bus.Pub(ctx, eventbus.Event{EventType: "OrderPlaced", Payload: order})
```

| Backend        | Scenario                                      |
| -------------- | --------------------------------------------- |
| Memory         | Unit testing, local development               |
| NATS JetStream | High throughput, event persistence and replay |
| RabbitMQ       | Enterprise message routing, complex topology  |
| Redis Pub/Sub  | Lightweight real-time notifications           |

### Domain-driven Design (DDD)

Built-in aggregate root template, event sourcing out-of-the-box:

```go
type Order struct {
    entity.BaseAggregateRoot
    Status string
}

func (o *Order) PlaceOrder(items []Item) {
    // Generate domain event, automatically track version
    o.AddDomainEvent(&OrderPlaced{Items: items})
    o.Status = "placed"
}
```

- Aggregate root version control & optimistic locking
- Snapshot mechanism accelerates large version aggregate reconstruction
- Command pattern and use case executor

### Generic Repository — Change Storage Without Changing Code

```go
// Declare interface, implementation auto-injected
type OrderRepo interface {
    usecase.Repository[Order]
}

// Switch from memory to PostgreSQL with just one DI line
```

| Implementation | Features                                                  |
| -------------- | --------------------------------------------------------- |
| Memory         | Zero dependencies, testing利器                            |
| PostgreSQL     | Full relational support, soft delete, pagination          |
| MongoDB        | Document-based, semi-structured data                      |
| Redis          | High-performance cache layer, TTL expiration              |
| BadgerDB       | Embedded KV, no additional deployment                     |
| GORM (SQL)     | Multi-database compatibility, complex relationship models |

### Task Scheduling System

Three scheduling strategies flexibly combined:

```go
// Concurrent scheduling — multiple tasks run simultaneously
scheduler := tasks.NewConcurrentScheduler()

// Sequential scheduling — strict order preservation
scheduler := tasks.NewSequentialScheduler()

// Hybrid scheduling — concurrent within batches, ordered between batches
scheduler := tasks.NewHybridScheduler()
```

### CQRS Projection System

Based on NATS JetStream full & incremental projection, automatically rebuild event streams into read models:

```
Event Stream → Aggregate Root Reconstruction → Projection Calculation → View Storage
```

### Saga Coordinator

Orchestration hub for cross-service distributed transactions, compensation transactions one-click rollback.

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────┐
│                   HTTP / WebSocket                   │
│          Gin Router · JWT · Casbin RBAC              │
├─────────────────────────────────────────────────────┤
│                    Application Use Case Layer        │
│        Command Handler · Query · Saga · Executor     │
├──────────────────────┬──────────────────────────────┤
│       Domain Model    │         Event-driven        │
│  Entity · AggRoot    │  EventBus · EventStore        │
│  DomainEvent · Cmd   │  Publisher · Subscriber       │
├──────────────────────┴──────────────────────────────┤
│                    Infrastructure Layer              │
│   PostgreSQL · MongoDB · Redis · BadgerDB · GORM     │
│   NATS JetStream · RabbitMQ · Zap · Viper            │
└─────────────────────────────────────────────────────┘
```

---

## Quick Start

**Environment Requirements**: Go 1.25+

```bash
# Clone project
git clone https://github.com/DotNetAge/sparrow.git
cd sparrow/src

# Install dependencies
make deps

# Run tests
make test

# Start service
make run
```

**Using Docker**:

```bash
# Build image
make docker-build

# Start full service (including dependencies)
make docker-run
```

---

## Tech Stack

| Domain                  | Technology                              |
| ----------------------- | --------------------------------------- |
| Language                | Go 1.25                                 |
| Web Framework           | Gin v1.11                               |
| Permission Control      | Casbin v2 (RBAC)                        |
| Authentication          | JWT (golang-jwt/jwt v5) + RSA           |
| ORM                     | GORM v1.31 · sqlx v1.4                  |
| Database                | PostgreSQL · MongoDB · SQLite           |
| Cache                   | Redis (go-redis v8/v9)                  |
| Embedded Storage        | BadgerDB v4                             |
| Message Queue           | NATS JetStream v1.46 · RabbitMQ amqp091 |
| Logging                 | Zap v1.27 + Lumberjack rolling logs     |
| Configuration           | Viper v1.21                             |
| Real-time Communication | Gorilla WebSocket v1.5                  |
| Testing                 | Testify · Testcontainers                |

---

## Project Structure

```
pkg/
├── entity/          # Domain model: aggregate root, entity, domain event, command
├── usecase/         # Use case layer: repository interface, event store interface, executor
├── eventbus/        # Event bus: Memory / NATS / RabbitMQ / Redis
├── messaging/       # Messaging subsystem: JetStream pub-sub, event serialization
├── persistence/     # Repository implementation: multi-data source adaptation
│   └── repo/        # PostgreSQL · MongoDB · Redis · BadgerDB · GORM · Memory
├── projection/      # Projection system: full projection, JetStream indexer
├── tasks/           # Task scheduling: concurrent / sequential / hybrid scheduler
├── auth/            # Authentication and authorization: JWT issuance, RSA verification
├── bootstrap/       # Application startup: DI container, lifecycle management
├── config/          # Configuration management: multi-format, multi-source
├── adapter/         # Interface adapters
│   ├── http/        # Gin router, Handler, middleware
│   ├── projection/  # Projection adapter
│   └── saga/        # Saga coordinator
├── logger/          # Logging: Zap + rolling files
└── ws/              # WebSocket adapter
```

---

## Use Cases

- Financial / e-commerce / logistics business systems requiring **auditability, replayability**
- **Cross-service distributed transaction** coordination under microservice architecture
- Medium to large teams with high requirements for **testability, maintainability**
- Projects wanting to落地 DDD but not wanting to build infrastructure from scratch

---

## Development Guide

```bash
make fmt      # Format code
make lint     # Run golangci-lint
make test     # Run all tests
make build    # Compile binary
```

---

<div align="center">

**Sparrow** — Small body, big energy. Make complex distributed systems simple again.

</div>

