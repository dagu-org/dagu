package frontend

import (
	"embed"
)

var (
	//go:embed templates/* assets/*
	assetsFS embed.FS
)
