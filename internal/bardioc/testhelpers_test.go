package bardioc

import (
	"encoding/json"
	"errors"
)

// fakeRows is an in-memory graph.Rows implementation for unit tests. It
// round-trips each item through JSON, matching how the real ws.Client scans
// HTTP response bodies.
type fakeRows struct {
	items [][]byte
	idx   int
	err   error
}

func newFakeRows(items ...any) *fakeRows {
	r := &fakeRows{}
	for _, item := range items {
		b, err := json.Marshal(item)
		if err != nil {
			r.err = err
			return r
		}
		r.items = append(r.items, b)
	}
	return r
}

func (r *fakeRows) Next() bool { return r.idx < len(r.items) }

func (r *fakeRows) Scan(dest any) error {
	if r.idx >= len(r.items) {
		return errors.New("fakeRows: no more rows")
	}
	err := json.Unmarshal(r.items[r.idx], dest)
	r.idx++
	return err
}

func (r *fakeRows) Close() {}

func (r *fakeRows) Err() error { return r.err }

// fakeRow is an in-memory graph.Row implementation for unit tests.
type fakeRow struct {
	data []byte
	err  error
}

func newSingleRow(item any) *fakeRow {
	b, err := json.Marshal(item)
	if err != nil {
		return &fakeRow{err: err}
	}
	return &fakeRow{data: b}
}

func (r *fakeRow) Scan(dest any) error {
	if r.err != nil {
		return r.err
	}
	return json.Unmarshal(r.data, dest)
}

func (r *fakeRow) Err() error { return r.err }

func (r *fakeRow) Close() {}
