---
name: GitHub Actions CI Design
description: CI workflow for running tests on pull_request to master
type: project
---

# GitHub Actions CI — Design

## Summary

Add a single GitHub Actions workflow that runs tests on every pull request targeting `master`.

## Trigger

- `pull_request` targeting `master`

## Runner

`ubuntu-latest` — Docker is available out of the box, required for TestContainers (PostgreSQL).

## Steps

1. `actions/checkout@v4` — check out the code
2. `actions/setup-go@v5` with `go-version: '1.26'` — install Go
3. `go test -v -race ./...` — run all tests including integration tests via TestContainers

## Configuration

- File: `.github/workflows/ci.yml`
- Job timeout: 10 minutes
- No caching (simplicity over speed)
