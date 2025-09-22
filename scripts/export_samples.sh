#!/usr/bin/env bash
go run ./cmd/usdewatcher --config ./config.yaml export \
    --from "2025-09-22T02:00:00Z" \
    --to   "2025-12-23T00:00:00Z" \
    --png ./out/usde-susde.png \
    --csv ./out/usde-susde.csv
