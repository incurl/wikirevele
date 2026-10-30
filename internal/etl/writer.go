package etl

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
)

// WriteBufferSize is the size of the buffer in front of the output
// files.
//
// The dump produces a lot of small records, and the writes are the
// bottleneck, so the buffer is large enough to absorb them.
const WriteBufferSize = 256 << 10

// EntityWriter writes entities as NDJSON.
//
// One record per line: a JSON object on each line, so that the file
// can be read a line at a time and imported without holding it in
// memory.
type EntityWriter struct {
	// out is the buffered writer the records go to.
	out *bufio.Writer
	// encoder writes the JSON records.
	encoder *json.Encoder
	// count is the number of records written.
	count int64
}

// NewEntityWriter creates an entity writer that writes to w.
//
// The caller owns the writer and is responsible for flushing it.
func NewEntityWriter(w io.Writer) *EntityWriter {
	out := bufio.NewWriterSize(w, WriteBufferSize)
	encoder := json.NewEncoder(out)

	return &EntityWriter{out: out, encoder: encoder}
}

// Write writes one entity.
//
// It returns an error when the record cannot be written, which is a
// failure of the output and not of the entity: the caller decides
// whether to stop.
func (w *EntityWriter) Write(entity Entity) error {
	// The encoder adds the newline that makes the file NDJSON.
	if err := w.encoder.Encode(entity); err != nil {
		return err
	}
	w.count++
	return nil
}

// Count returns the number of records written so far.
func (w *EntityWriter) Count() int64 {
	return w.count
}

// Flush writes the buffered records to the underlying writer.
func (w *EntityWriter) Flush() error {
	if err := w.out.Flush(); err != nil {
		return fmt.Errorf("flush the records: %w", err)
	}
	return nil
}
