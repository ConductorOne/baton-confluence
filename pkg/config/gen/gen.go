package main

import (
	cfg "github.com/conductorone/baton-confluence/pkg/config"
	"github.com/conductorone/baton-sdk/pkg/config"
)

func main() {
	config.Generate("confluence", cfg.Configuration)
}
