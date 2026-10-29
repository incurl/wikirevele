package jobs

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

// TestDescribeErrorUnwrapsARegularChain verifies that a coded error is
// found through a fmt.Errorf wrap.
func TestDescribeErrorUnwrapsARegularChain(t *testing.T) {
	cause := &Error{Code: 42, Message: "the API refused the request"}

	code, message := describeError(fmt.Errorf("call the API: %w", cause))
	if code != 42 {
		t.Errorf("code = %d, want 42", code)
	}
	if message != "the API refused the request" {
		t.Errorf("message = %q", message)
	}
}

// TestDescribeErrorUnwrapsAJoinedError verifies that a coded error is
// found inside errors.Join.
//
// This is the regression test for the finding of the 2026-10-28 code
// review: the hand-rolled unwrap loop only understood Unwrap() error, so
// errors.Join's Unwrap() []error hid the code. A runner that reports the
// API failure and a cleanup failure together lost the code entirely.
func TestDescribeErrorUnwrapsAJoinedError(t *testing.T) {
	cause := &Error{Code: 7, Message: "the API refused the request"}
	joined := errors.Join(cause, errors.New("the cleanup failed"))

	code, message := describeError(joined)
	if code != 7 {
		t.Errorf("code = %d, want 7: the code was lost through errors.Join", code)
	}
	if message != "the API refused the request" {
		t.Errorf("message = %q", message)
	}
}

// TestDescribeErrorReportsAPlainError verifies that an error with no
// code is reported with its own text and no code.
func TestDescribeErrorReportsAPlainError(t *testing.T) {
	code, message := describeError(errors.New("something broke"))
	if code != 0 {
		t.Errorf("code = %d, want 0", code)
	}
	if message != "something broke" {
		t.Errorf("message = %q", message)
	}
}

// TestJoinedErrorCodeReachesTheJob verifies the same thing through a
// whole job.
func TestJoinedErrorCodeReachesTheJob(t *testing.T) {
	m := newTestManager(t, WithRunner(RunnerFunc(
		func(ctx context.Context, job Job) (*Result, error) {
			return nil, errors.Join(
				&Error{Code: 7, Message: "the API refused the request"},
				errors.New("the cleanup failed"),
			)
		},
	)))

	job, err := m.Submit(Request{Prompt: "a job"})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}

	finished := waitForStatus(t, m, job.ID, StatusFailed)
	if finished.ErrorCode != 7 {
		t.Errorf("error code = %d, want 7", finished.ErrorCode)
	}
}
