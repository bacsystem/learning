package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestParseArgs(t *testing.T) {
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	o, err := parseArgs(nil, env(map[string]string{"DATABASE_URL": "x", "GOOSE_BIN": "/bin/g"}))
	if err != nil {
		t.Fatal(err)
	}
	if o.goose != "/bin/g" || o.dir != "./migrations" || !reflect.DeepEqual(o.args, []string{"up"}) {
		t.Fatalf("bad defaults: %+v", o)
	}
	o, err = parseArgs([]string{"-dir", "m", "down-to", "3"}, env(map[string]string{"DATABASE_URL": "x"}))
	if err != nil || o.dir != "m" || !reflect.DeepEqual(o.args, []string{"down-to", "3"}) {
		t.Fatalf("bad parse: %+v %v", o, err)
	}
	if _, err := parseArgs(nil, env(nil)); err == nil {
		t.Fatal("DATABASE_URL must be required")
	}
}

// TestRunSerializesConcurrentRuns runs two migrate invocations at once with a
// fake goose that records when it starts/ends and sleeps; with the advisory
// lock the two critical sections must not overlap.
func TestRunSerializesConcurrentRuns(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}
	dir := t.TempDir()
	logf := filepath.Join(dir, "events")
	fake := filepath.Join(dir, "goose")
	script := "#!/bin/sh\n" +
		"[ \"$GOOSE_DRIVER\" = postgres ] && [ -n \"$GOOSE_DBSTRING\" ] || exit 9\n" +
		"echo start >> " + logf + "\nsleep 0.5\necho end >> " + logf + "\n"
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			var out bytes.Buffer
			errs[i] = run(context.Background(), options{
				dsn: dsn, dir: dir, goose: fake, lockTimeout: 30 * time.Second, args: []string{"up"},
			}, &out, &out)
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("run %d: %v", i, err)
		}
	}
	b, err := os.ReadFile(logf)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Fields(string(b)); !reflect.DeepEqual(got, []string{"start", "end", "start", "end"}) {
		t.Fatalf("critical sections overlapped: %v", got)
	}
}

func TestRunPropagatesGooseFailure(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}
	fake := filepath.Join(t.TempDir(), "goose")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\nexit 3\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := run(context.Background(), options{dsn: dsn, dir: ".", goose: fake, lockTimeout: time.Second, args: []string{"up"}}, &out, &out); err == nil {
		t.Fatal("expected goose failure to propagate")
	}
}
