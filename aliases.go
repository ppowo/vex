package main

// Alias → real environment variable name.
// Add new entries here and rebuild.
var aliases = map[string]string{
	"aws":    "AWS_PROFILE",
	"region": "AWS_REGION",
	"node":   "NODE_ENV",
}
