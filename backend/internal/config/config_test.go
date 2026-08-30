package config

import "testing"

func TestOverrideFromEnvDatabase(t *testing.T) {
	originalConfig := AppConfig
	t.Cleanup(func() {
		AppConfig = originalConfig
	})

	t.Setenv("BLOG_DATABASE_DRIVER", " MySQL ")
	t.Setenv("BLOG_DATABASE_DSN", "user:password@tcp(example.com:4000)/blog")

	AppConfig = &Config{
		Database: Database{
			Driver: "sqlite",
			DSN:    "blog.db",
		},
	}

	overrideFromEnv()

	if AppConfig.Database.Driver != "mysql" {
		t.Fatalf("expected mysql driver, got %q", AppConfig.Database.Driver)
	}

	if AppConfig.Database.DSN != "user:password@tcp(example.com:4000)/blog" {
		t.Fatalf("database DSN was not overridden: %q", AppConfig.Database.DSN)
	}
}
