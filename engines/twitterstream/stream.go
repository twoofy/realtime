// Copyright 2010 Gary Burd
//
// Licensed under the Apache License, Version 2.0 (the "License"): you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations
// under the License.

// Package twitterstream implements the basic functionality for accessing the
// Twitter streaming APIs. See http://dev.twitter.com/pages/streaming_api for
// information on the Twitter streaming APIs.
//
// The following example shows how to handle dropped connections. If it's
// important for the application to see every tweet in the stream, then the
// application should backfill the stream using the Twitter search API after
// each connection attempt.
//
//  waitUntil := time.Now()
//  for {
//      // Rate limit connection attempts to once every 30 seconds.
//      if d := waitUntil.Sub(time.Now()); d > 0 {
//          time.Sleep(d)
//      }
//      waitUntil = time.Now().Add(30 * time.Second)
//
//      ts, err := twitterstream.Open(client, cred, url, params)
//      if err != nil {
//          log.Println("error opening stream: ", err)
//          continue
//      }
//
//      // Loop until stream has a permanent error.
//      for ts.Err() == nil {
//          var t MyTweet
//          if err := ts.UnmarshalNext(&t); err != nil {
//              log.Println("error reading tweet: ", err)
//              continue
//          }
//          process(&t)
//      }
//      ts.Close()
//  }
//
package twitterstream

// Implementation notes:
//
// This package uses low-level functions from net/http to issue HTTP requests
// because it's impossible to manage connection timeouts using the high-level
// net/http Client API.
//
// It would be nice if the API for this package matched the new bufio Scanner
// type. It's not possible to change the API without breaking existing uses of
// the package.

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/tls"
	"encoding/json"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"engines/github.com.garyburd.go-oauth/oauth"
)

// Stream manages the connection to a Twitter streaming endpoint.
type Stream struct {
	conn net.Conn
	r    *bufio.Scanner
	err  error
}

// HTTPStatusError represents an HTTP error return from the Twitter streaming
// API endpoint.
type HTTPStatusError struct {
	// HTTP status code.
	StatusCode int

	// Response body.
	Message string
}

func (err HTTPStatusError) Error() string {
	return "twitterstream: status=" + strconv.Itoa(err.StatusCode) + " " + err.Message
}

var (
	responseLineRegexp = regexp.MustCompile("^HTTP/[0-9.]+ ([0-9]+) ")
	crlf               = []byte("\r\n")
)

// Open opens a new stream.
func Open(oauthClient *oauth.Client, accessToken *oauth.Credentials, urlStr string, params url.Values) (*Stream, error) {
	d := net.Dialer{
		Timeout: time.Minute,
	}
	return openInternal(d.Dial, oauthClient, accessToken, urlStr, params)
}

func openInternal(dial func(network, address string) (net.Conn, error),
	oauthClient *oauth.Client,
	accessToken *oauth.Credentials,
	urlStr string,
	params url.Values) (*Stream, error) {

	paramsStr := params.Encode()
	req, err := http.NewRequest("POST", urlStr, strings.NewReader(paramsStr))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", oauthClient.AuthorizationHeader(accessToken, "POST", req.URL, params))
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Content-Length", strconv.Itoa(len(paramsStr)))

	host := req.URL.Host
	port := "80"
	if h, p, err := net.SplitHostPort(req.URL.Host); err == nil {
		host = h
		port = p
	} else {
		if req.URL.Scheme == "https" {
			port = "443"
		}
	}

	ts := &Stream{}
	ts.conn, err = dial("tcp", host+":"+port)
	if err != nil {
		return nil, err
	}
	ts.conn.(*net.TCPConn).SetLinger(60)

	if req.URL.Scheme == "https" {
		conn := tls.Client(ts.conn, &tls.Config{ServerName: host})
		ts.conn = conn
		if err := conn.Handshake(); err != nil {
			return nil, ts.fatal(err)
		}
		if err := conn.VerifyHostname(host); err != nil {
			return nil, ts.fatal(err)
		}
	}

	err = ts.conn.SetDeadline(time.Now().Add(60 * time.Second))
	if err != nil {
		return nil, ts.fatal(err)
	}

	if err := req.Write(ts.conn); err != nil {
		return nil, ts.fatal(err)
	}

	br := bufio.NewReader(ts.conn)
	resp, err := http.ReadResponse(br, req)
	if err != nil {
		return nil, ts.fatal(err)
	}

	if resp.Header.Get("Content-Encoding") == "gzip" {
		rc, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, ts.fatal(err)
		}
		resp.Body = rc
	}

	if resp.StatusCode != 200 {
		p, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, ts.fatal(err)
		}
		return nil, ts.fatal(HTTPStatusError{resp.StatusCode, string(p)})
	}

	ts.conn.SetWriteDeadline(time.Time{})

	ts.r = bufio.NewScanner(resp.Body)
	ts.r.Split(splitLines)
	return ts, nil
}

func (ts *Stream) fatal(err error) error {
	if ts.conn != nil {
		ts.conn.Close()
	}
	if ts.err == nil {
		ts.err = err
	}
	return err
}

// Close releases the resources used by the stream. It can be called
// concurrently with Next.
func (ts *Stream) Close() error {
	return ts.conn.Close()
}

// Err returns a non-nil value if the stream has a permanent error.
func (ts *Stream) Err() error {
	return ts.err
}

// Next returns the next line from the stream. The returned slice is
// overwritten by the next call to Next.
func (ts *Stream) Next() ([]byte, error) {
	if ts.err != nil {
		return nil, ts.err
	}
	for {
		// Twitter recommends reading with a timeout of 90 seconds.
		err := ts.conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		if err != nil {
			return nil, ts.fatal(err)
		}
		if !ts.r.Scan() {
			err := ts.r.Err()
			if err == nil {
				err = io.EOF
			}
			return nil, ts.fatal(err)
		}
		p := ts.r.Bytes()
		if len(p) > 0 {
			return p, nil
		}
	}
}

// UnmarshalNext reads the next line of from the stream and decodes the line as
// JSON to data. This is a convenience function for streams with homogeneous
// entity types.
func (ts *Stream) UnmarshalNext(data interface{}) error {
	p, err := ts.Next()
	if err != nil {
		return err
	}
	return json.Unmarshal(p, data)
}

func splitLines(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.Index(data, crlf); i >= 0 {
		// We have a full CRLF terminated line.
		return i + 2, data[:i], nil
	}
	if atEOF {
		return 0, nil, io.ErrUnexpectedEOF
	}
	// Request more data.
	return 0, nil, nil
}
