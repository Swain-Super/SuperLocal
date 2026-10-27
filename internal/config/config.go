package config

import (
	"flag"
	"fmt"
	"strconv"
	"strings"
)

type Config struct {
	Addr      string
	DataDir   string
	MaxUpload int64
}

func FromArgs(args []string) (Config, error) {
	fs := flag.NewFlagSet("superlocal", flag.ContinueOnError)
	addr := fs.String("addr", ":8080", "listen address")
	data := fs.String("data", "./data", "data directory")
	maxUpload := fs.String("max-upload", "200MB", "max upload size")
	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}
	n, err := parseSize(*maxUpload)
	if err != nil {
		return Config{}, err
	}
	return Config{Addr: *addr, DataDir: *data, MaxUpload: n}, nil
}

func parseSize(raw string) (int64, error) {
	s := strings.TrimSpace(strings.ToUpper(raw))
	mult := int64(1)
	switch {
	case strings.HasSuffix(s, "MB"):
		mult = 1024 * 1024
		s = strings.TrimSuffix(s, "MB")
	case strings.HasSuffix(s, "KB"):
		mult = 1024
		s = strings.TrimSuffix(s, "KB")
	}
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("无效的大小 %q", raw)
	}
	return n * mult, nil
}
