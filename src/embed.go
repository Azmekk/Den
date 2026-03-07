package src

import "embed"

//go:embed web/build/*
var StaticFiles embed.FS
