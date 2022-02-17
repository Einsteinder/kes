// Copyright 2019 - MinIO, Inc. All rights reserved.
// Use of this source code is governed by the AGPLv3
// license that can be found in the LICENSE file.

package kes

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"time"
)

// ErrorEvent is the event a KES server generates when
// it encounters and logs an error.
type ErrorEvent struct {
	Message string // The logged error message
}

// NewErrorStream returns a new ErrorStream that
// reads from r.
func NewErrorStream(r io.Reader) *ErrorStream {
	s := &ErrorStream{
		decoder: json.NewDecoder(r),
	}
	if closer, ok := r.(io.Closer); ok {
		s.closer = closer
	}
	return s
}

// ErrorStream iterates over a stream of ErrorEvents.
// Close the ErrorStream to release associated resources.
type ErrorStream struct {
	decoder *json.Decoder
	closer  io.Closer

	event  ErrorEvent
	err    error
	closed bool
}

// Event returns the most recent ErrorEvent, generated by Next.
func (s *ErrorStream) Event() ErrorEvent { return s.event }

// Message returns the current error message or the ErrorEvent.
// It is a short-hand for Event().Message.
func (s *ErrorStream) Message() string { return s.event.Message }

// Next advances the stream to the next ErrorEvent and
// returns true if there is another one. It returns
// false if there are no more ErrorEvents or when
// the ErrorStream encountered an error.
func (s *ErrorStream) Next() bool {
	type Response struct {
		Message string `json:"message"`
	}
	if s.err != nil || s.closed {
		return false
	}

	var resp Response
	if err := s.decoder.Decode(&resp); err != nil {
		if errors.Is(err, io.EOF) {
			s.err = s.Close()
		} else {
			s.err = err
		}
		return false
	}
	s.event = ErrorEvent(resp)
	return true
}

// WriteTo writes the entire ErrorEvent stream to w.
// It returns the number of bytes written to w and
// the first error encounterred, if any.
func (s *ErrorStream) WriteTo(w io.Writer) (int64, error) {
	type Response struct {
		Message string `json:"message"`
	}

	cw := countWriter{W: w}
	encoder := json.NewEncoder(&cw)
	for {
		var resp Response
		if err := s.decoder.Decode(&resp); err != nil {
			if errors.Is(err, io.EOF) {
				s.err = s.Close()
			} else {
				s.err = err
			}
			return cw.N, s.err
		}
		if err := encoder.Encode(resp); err != nil {
			s.err = err
			return cw.N, err
		}
	}
}

// Close closes the ErrorStream and releases
// any associated resources.
func (s *ErrorStream) Close() error {
	if !s.closed {
		s.closed = true

		if s.closer != nil {
			err := s.closer.Close()
			if s.err == nil {
				s.err = err
			}
			return err
		}
	}
	return s.err
}

// AuditEvent is the event a KES server generates when
// it response to a request.
type AuditEvent struct {
	Timestamp time.Time // The point in time when the KES server received the request
	APIPath   string    // The API called by the client. May contain API arguments

	ClientIP       net.IP   // The client's IP address
	ClientIdentity Identity // The client's KES identity

	StatusCode   int           // The response status code sent to the client
	ResponseTime time.Duration // Time it took to process the request
}

// NewAuditStream returns a new AuditStream that
// reads from r.
func NewAuditStream(r io.Reader) *AuditStream {
	s := &AuditStream{
		decoder: json.NewDecoder(r),
	}
	if closer, ok := r.(io.Closer); ok {
		s.closer = closer
	}
	return s
}

// AuditStream iterates over a stream of AuditEvents.
// Close the AuditStream to release associated resources.
type AuditStream struct {
	decoder *json.Decoder
	closer  io.Closer

	event  AuditEvent
	err    error
	closed bool
}

// Event returns the most recent AuditEvent, generated by Next.
func (s *AuditStream) Event() AuditEvent { return s.event }

// Next advances the stream to the next AuditEvent and
// returns true if there is another one. It returns
// false if there are no more AuditEvents or when the
// AuditStream encountered an error.
func (s *AuditStream) Next() bool {
	type Response struct {
		Timestamp time.Time `json:"time"`
		Request   struct {
			IP       net.IP   `json:"ip"`
			APIPath  string   `json:"path"`
			Identity Identity `json:"identity"`
		} `json:"request"`
		Response struct {
			StatusCode int           `json:"code"`
			Time       time.Duration `json:"time"`
		} `json:"response"`
	}
	if s.closed || s.err != nil {
		return false
	}
	var resp Response
	if err := s.decoder.Decode(&resp); err != nil {
		if errors.Is(err, io.EOF) {
			s.err = s.Close()
		} else {
			s.err = err
		}
		return false
	}
	s.event = AuditEvent{
		Timestamp:      resp.Timestamp,
		APIPath:        resp.Request.APIPath,
		ClientIP:       resp.Request.IP,
		ClientIdentity: resp.Request.Identity,
		StatusCode:     resp.Response.StatusCode,
		ResponseTime:   resp.Response.Time,
	}
	return true
}

// WriteTo writes the entire AuditEvent stream to w.
// It returns the number of bytes written to w and
// the first error encountered, if any.
func (s *AuditStream) WriteTo(w io.Writer) (int64, error) {
	type Response struct {
		Timestamp time.Time `json:"time"`
		Request   struct {
			IP       net.IP   `json:"ip"`
			APIPath  string   `json:"path"`
			Identity Identity `json:"identity"`
		} `json:"request"`
		Response struct {
			StatusCode int           `json:"code"`
			Time       time.Duration `json:"time"`
		} `json:"response"`
	}

	cw := countWriter{W: w}
	encoder := json.NewEncoder(&cw)
	for {
		var resp Response
		if err := s.decoder.Decode(&resp); err != nil {
			if errors.Is(err, io.EOF) {
				s.err = s.Close()
			} else {
				s.err = err
			}
			return cw.N, s.err
		}
		if err := encoder.Encode(resp); err != nil {
			s.err = err
			return cw.N, err
		}
	}
}

// Close closes the AuditStream and releases
// any associated resources.
func (s *AuditStream) Close() (err error) {
	if !s.closed {
		s.closed = true

		if s.closer != nil {
			err := s.closer.Close()
			if s.err == nil {
				s.err = err
			}
			return err
		}
	}
	return s.err
}

type countWriter struct {
	W io.Writer
	N int64
}

func (w *countWriter) Write(p []byte) (int, error) {
	n, err := w.W.Write(p)
	w.N += int64(n)
	return n, err
}
