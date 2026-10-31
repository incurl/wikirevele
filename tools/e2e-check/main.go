// A live end-to-end check: submit a job against a stub runner and
// confirm that the job is persisted, executed and served.
//
// It exercises the whole pipeline without a GUI: the PocketBase schema
// is applied, the job is queued, the runner answers and the artifact is
// stored and served by PocketBase's own file route.
package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"time"

	"github.com/incurl/wikirevele/internal/jobs"
	wikirevelepb "github.com/incurl/wikirevele/internal/pocketbase"
)

func main() {
	dir, err := os.MkdirTemp("", "wikirevele-e2e")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	svc := wikirevelepb.NewService(dir)
	if err := svc.Bootstrap(); err != nil {
		panic(err)
	}
	defer svc.Close()
	fmt.Println("1. PocketBase bootstrapped, schema applied")

	// A stub runner that stands in for a real backend.
	var received jobs.Job
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer stub.Close()

	manager := jobs.NewManager(svc.App(),
		jobs.WithRunner(jobs.RunnerFunc(
			func(ctx context.Context, job jobs.Job) (*jobs.Result, error) {
				received = job
				return &jobs.Result{
					Payload: map[string]any{"echo": job.Prompt},
				}, nil
			},
		)),
	)

	job, err := manager.Submit(jobs.Request{
		Title:  "a red bicycle",
		Prompt: "a red bicycle",
		Params: map[string]any{"style": "photo"},
	})
	if err != nil {
		panic(err)
	}
	fmt.Printf("2. Job submitted: %s (status %s)\n", job.ID, job.Status)

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		current, _ := manager.GetJob(job.ID)
		if current.Status == "completed" || current.Status == "failed" {
			job = current
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	fmt.Printf("3. Job finished: status=%s artifacts=%d\n", job.Status, len(job.ArtifactIDs))
	if job.Status != "completed" {
		fmt.Println("   error:", job.ErrorMessage)
		os.Exit(1)
	}

	// The runner must have received the prompt and the parameters.
	if received.Prompt != "a red bicycle" {
		fmt.Printf("   FAIL: the runner received %q\n", received.Prompt)
		os.Exit(1)
	}
	if received.Params["style"] != "photo" {
		fmt.Printf("   FAIL: the runner received the params %v\n", received.Params)
		os.Exit(1)
	}
	fmt.Println("   the runner received the prompt and the parameters")

	// The result must have been stored.
	if job.Result["echo"] != "a red bicycle" {
		fmt.Printf("   FAIL: the stored result is %v\n", job.Result)
		os.Exit(1)
	}
	fmt.Println("   the result was stored on the job")

	// An artifact must be stored and served by PocketBase's native
	// route.
	document := []byte(`{"kind":"dataset","rows":42}`)
	artifact, err := manager.StoreArtifact(job.ID, jobs.ArtifactKindOutput, document, "dataset.json", nil)
	if err != nil {
		panic(err)
	}
	fmt.Printf("4. Artifact stored: %s (%s, %d bytes)\n", artifact.URL, artifact.Mime, artifact.Bytes)

	handler, err := svc.Handler()
	if err != nil {
		panic(err)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, artifact.URL, nil))
	fmt.Printf("5. File route: GET %s -> %d (%d bytes, %s)\n",
		artifact.URL, rec.Code, rec.Body.Len(), rec.Header().Get("Content-Type"))
	if rec.Code != 200 || rec.Body.Len() == 0 {
		os.Exit(1)
	}

	fmt.Println("\nEND TO END: PASS")
}
