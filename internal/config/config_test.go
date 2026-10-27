package config

import "testing"

func TestFromArgsDefaults(t *testing.T) {
	cfg, err := FromArgs([]string{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != ":8080" || cfg.DataDir != "./data" || cfg.MaxUpload != 200*1024*1024 {
		t.Fatalf("%+v", cfg)
	}
}

func TestFromArgsOverride(t *testing.T) {
	cfg, err := FromArgs([]string{"-addr", "127.0.0.1:9", "-data", "/tmp/sl", "-max-upload", "10MB"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != "127.0.0.1:9" || cfg.DataDir != "/tmp/sl" || cfg.MaxUpload != 10*1024*1024 {
		t.Fatalf("%+v", cfg)
	}
}
