package main

import "embed"

//go:embed web/build/*
var StaticFiles embed.FS

//go:embed db/migrations/*.sql
var MigrationFiles embed.FS
