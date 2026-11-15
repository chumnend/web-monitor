# Web  Monitor

## About

**Web Monitor** is a full-stack web application designed to monitor websites and services in real-time, providing clear visibility into uptime and availability.

## Prerequisites

Before getting started, make sure you have the following installed on your system:
- [Docker](https://docs.docker.com/get-docker/) & [Docker Compose](https://docs.docker.com/compose/)
- [Node.js](https://nodejs.org/) & [Yarn](https://yarnpkg.com/) (only required if running the frontend locally for development)

## Getting Started

### Option 1: Quick Start with Docker (Recommended)

Launch all services,

```
make up
```

### Option 2: Local Development

Run the backend,

```
cd app/backend
cp .env.example .env
# Edit .env and configure your environment variables
make
```

Run the frontend,

```
cd app/frontend
yarn install
yarn dev
```

## Documentation

- [API Reference & Usage Guide](docs/API.md)
- [Technical Specification](docs/SPEC.md)

## Contact

Nicholas Chumney - [nicholas.chumney@outlook.com](nicholas.chumney@outlook.com)
