package web

import "embed"

//go:embed static/index.html static/app.css static/app.js
var assets embed.FS
