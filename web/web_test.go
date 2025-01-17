// Copyright 2016 The dnxware Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package web

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dnxware/dnxware/config"
	"github.com/dnxware/dnxware/notifier"
	"github.com/dnxware/dnxware/rules"
	"github.com/dnxware/dnxware/scrape"
	"github.com/dnxware/dnxware/storage/tsdb"
	"github.com/dnxware/dnxware/util/testutil"
	libtsdb "github.com/dnxware/tsdb"
)

func TestMain(m *testing.M) {
	// On linux with a global proxy the tests will fail as the go client(http,grpc) tries to connect through the proxy.
	os.Setenv("no_proxy", "localhost,127.0.0.1,0.0.0.0,:")
	os.Exit(m.Run())
}
func TestGlobalURL(t *testing.T) {
	opts := &Options{
		ListenAddress: ":7071",
		ExternalURL: &url.URL{
			Scheme: "https",
			Host:   "externalhost:80",
			Path:   "/path/prefix",
		},
	}

	tests := []struct {
		inURL  string
		outURL string
	}{
		{
			// Nothing should change if the input URL is not on localhost, even if the port is our listening port.
			inURL:  "http://somehost:7071/metrics",
			outURL: "http://somehost:7071/metrics",
		},
		{
			// Port and host should change if target is on localhost and port is our listening port.
			inURL:  "http://localhost:7071/metrics",
			outURL: "https://externalhost:80/metrics",
		},
		{
			// Only the host should change if the port is not our listening port, but the host is localhost.
			inURL:  "http://localhost:8000/metrics",
			outURL: "http://externalhost:8000/metrics",
		},
		{
			// Alternative localhost representations should also work.
			inURL:  "http://127.0.0.1:7071/metrics",
			outURL: "https://externalhost:80/metrics",
		},
	}

	for _, test := range tests {
		inURL, err := url.Parse(test.inURL)

		testutil.Ok(t, err)

		globalURL := tmplFuncs("", opts)["globalURL"].(func(u *url.URL) *url.URL)
		outURL := globalURL(inURL)

		testutil.Equals(t, test.outURL, outURL.String())
	}
}

func TestReadyAndHealthy(t *testing.T) {
	t.Parallel()
	dbDir, err := ioutil.TempDir("", "tsdb-ready")

	testutil.Ok(t, err)

	defer os.RemoveAll(dbDir)
	db, err := libtsdb.Open(dbDir, nil, nil, nil)

	testutil.Ok(t, err)

	opts := &Options{
		ListenAddress:  ":7071",
		ReadTimeout:    30 * time.Second,
		MaxConnections: 512,
		Context:        nil,
		Storage:        &tsdb.ReadyStorage{},
		QueryEngine:    nil,
		ScrapeManager:  &scrape.Manager{},
		RuleManager:    &rules.Manager{},
		Notifier:       nil,
		RoutePrefix:    "/",
		EnableAdminAPI: true,
		TSDB:           func() *libtsdb.DB { return db },
		ExternalURL: &url.URL{
			Scheme: "http",
			Host:   "localhost:7071",
			Path:   "/",
		},
		Version: &PrometheusVersion{},
	}

	opts.Flags = map[string]string{}

	webHandler := New(nil, opts)

	webHandler.config = &config.Config{}
	webHandler.notifier = &notifier.Manager{}

	go func() {
		err := webHandler.Run(context.Background())
		if err != nil {
			panic(fmt.Sprintf("Can't start web handler:%s", err))
		}
	}()

	// Give some time for the web goroutine to run since we need the server
	// to be up before starting tests.
	time.Sleep(5 * time.Second)

	resp, err := http.Get("http://localhost:7071/-/healthy")

	testutil.Ok(t, err)
	testutil.Equals(t, http.StatusOK, resp.StatusCode)

	resp, err = http.Get("http://localhost:7071/-/ready")

	testutil.Ok(t, err)
	testutil.Equals(t, http.StatusServiceUnavailable, resp.StatusCode)

	resp, err = http.Get("http://localhost:7071/version")

	testutil.Ok(t, err)
	testutil.Equals(t, http.StatusServiceUnavailable, resp.StatusCode)

	resp, err = http.Get("http://localhost:7071/graph")

	testutil.Ok(t, err)
	testutil.Equals(t, http.StatusServiceUnavailable, resp.StatusCode)

	resp, err = http.Post("http://localhost:7071/api/v2/admin/tsdb/snapshot", "", strings.NewReader(""))

	testutil.Ok(t, err)
	testutil.Equals(t, http.StatusServiceUnavailable, resp.StatusCode)

	resp, err = http.Post("http://localhost:7071/api/v2/admin/tsdb/delete_series", "", strings.NewReader("{}"))

	testutil.Ok(t, err)
	testutil.Equals(t, http.StatusServiceUnavailable, resp.StatusCode)

	resp, err = http.Get("http://localhost:7071/graph")

	testutil.Ok(t, err)
	testutil.Equals(t, http.StatusServiceUnavailable, resp.StatusCode)

	resp, err = http.Get("http://localhost:7071/alerts")

	testutil.Ok(t, err)
	testutil.Equals(t, http.StatusServiceUnavailable, resp.StatusCode)

	resp, err = http.Get("http://localhost:7071/flags")

	testutil.Ok(t, err)
	testutil.Equals(t, http.StatusServiceUnavailable, resp.StatusCode)

	resp, err = http.Get("http://localhost:7071/rules")

	testutil.Ok(t, err)
	testutil.Equals(t, http.StatusServiceUnavailable, resp.StatusCode)

	resp, err = http.Get("http://localhost:7071/service-discovery")

	testutil.Ok(t, err)
	testutil.Equals(t, http.StatusServiceUnavailable, resp.StatusCode)

	resp, err = http.Get("http://localhost:7071/targets")

	testutil.Ok(t, err)
	testutil.Equals(t, http.StatusServiceUnavailable, resp.StatusCode)

	resp, err = http.Get("http://localhost:7071/config")

	testutil.Ok(t, err)
	testutil.Equals(t, http.StatusServiceUnavailable, resp.StatusCode)

	resp, err = http.Get("http://localhost:7071/status")

	testutil.Ok(t, err)
	testutil.Equals(t, http.StatusServiceUnavailable, resp.StatusCode)

	// Set to ready.
	webHandler.Ready()

	resp, err = http.Get("http://localhost:7071/-/healthy")

	testutil.Ok(t, err)
	testutil.Equals(t, http.StatusOK, resp.StatusCode)

	resp, err = http.Get("http://localhost:7071/-/ready")

	testutil.Ok(t, err)
	testutil.Equals(t, http.StatusOK, resp.StatusCode)

	resp, err = http.Get("http://localhost:7071/version")

	testutil.Ok(t, err)
	testutil.Equals(t, http.StatusOK, resp.StatusCode)

	resp, err = http.Get("http://localhost:7071/graph")

	testutil.Ok(t, err)
	testutil.Equals(t, http.StatusOK, resp.StatusCode)

	resp, err = http.Post("http://localhost:7071/api/v2/admin/tsdb/snapshot", "", strings.NewReader(""))

	testutil.Ok(t, err)
	testutil.Equals(t, http.StatusOK, resp.StatusCode)

	resp, err = http.Post("http://localhost:7071/api/v2/admin/tsdb/delete_series", "", strings.NewReader("{}"))

	testutil.Ok(t, err)
	testutil.Equals(t, http.StatusOK, resp.StatusCode)

	resp, err = http.Get("http://localhost:7071/alerts")

	testutil.Ok(t, err)
	testutil.Equals(t, http.StatusOK, resp.StatusCode)

	resp, err = http.Get("http://localhost:7071/flags")

	testutil.Ok(t, err)
	testutil.Equals(t, http.StatusOK, resp.StatusCode)

	resp, err = http.Get("http://localhost:7071/rules")

	testutil.Ok(t, err)
	testutil.Equals(t, http.StatusOK, resp.StatusCode)

	resp, err = http.Get("http://localhost:7071/service-discovery")

	testutil.Ok(t, err)
	testutil.Equals(t, http.StatusOK, resp.StatusCode)

	resp, err = http.Get("http://localhost:7071/targets")

	testutil.Ok(t, err)
	testutil.Equals(t, http.StatusOK, resp.StatusCode)

	resp, err = http.Get("http://localhost:7071/config")

	testutil.Ok(t, err)
	testutil.Equals(t, http.StatusOK, resp.StatusCode)

	resp, err = http.Get("http://localhost:7071/status")

	testutil.Ok(t, err)
	testutil.Equals(t, http.StatusOK, resp.StatusCode)
}

func TestRoutePrefix(t *testing.T) {
	t.Parallel()
	dbDir, err := ioutil.TempDir("", "tsdb-ready")

	testutil.Ok(t, err)

	defer os.RemoveAll(dbDir)

	db, err := libtsdb.Open(dbDir, nil, nil, nil)

	testutil.Ok(t, err)

	opts := &Options{
		ListenAddress:  ":9091",
		ReadTimeout:    30 * time.Second,
		MaxConnections: 512,
		Context:        nil,
		Storage:        &tsdb.ReadyStorage{},
		QueryEngine:    nil,
		ScrapeManager:  nil,
		RuleManager:    nil,
		Notifier:       nil,
		RoutePrefix:    "/dnxware",
		EnableAdminAPI: true,
		TSDB:           func() *libtsdb.DB { return db },
	}

	opts.Flags = map[string]string{}

	webHandler := New(nil, opts)
	go func() {
		err := webHandler.Run(context.Background())
		if err != nil {
			panic(fmt.Sprintf("Can't start web handler:%s", err))
		}
	}()

	// Give some time for the web goroutine to run since we need the server
	// to be up before starting tests.
	time.Sleep(5 * time.Second)

	resp, err := http.Get("http://localhost:9091" + opts.RoutePrefix + "/-/healthy")

	testutil.Ok(t, err)
	testutil.Equals(t, http.StatusOK, resp.StatusCode)

	resp, err = http.Get("http://localhost:9091" + opts.RoutePrefix + "/-/ready")

	testutil.Ok(t, err)
	testutil.Equals(t, http.StatusServiceUnavailable, resp.StatusCode)

	resp, err = http.Get("http://localhost:9091" + opts.RoutePrefix + "/version")

	testutil.Ok(t, err)
	testutil.Equals(t, http.StatusServiceUnavailable, resp.StatusCode)

	resp, err = http.Post("http://localhost:9091"+opts.RoutePrefix+"/api/v2/admin/tsdb/snapshot", "", strings.NewReader(""))

	testutil.Ok(t, err)
	testutil.Equals(t, http.StatusServiceUnavailable, resp.StatusCode)

	resp, err = http.Post("http://localhost:9091"+opts.RoutePrefix+"/api/v2/admin/tsdb/delete_series", "", strings.NewReader("{}"))

	testutil.Ok(t, err)
	testutil.Equals(t, http.StatusServiceUnavailable, resp.StatusCode)

	// Set to ready.
	webHandler.Ready()

	resp, err = http.Get("http://localhost:9091" + opts.RoutePrefix + "/-/healthy")

	testutil.Ok(t, err)
	testutil.Equals(t, http.StatusOK, resp.StatusCode)

	resp, err = http.Get("http://localhost:9091" + opts.RoutePrefix + "/-/ready")

	testutil.Ok(t, err)
	testutil.Equals(t, http.StatusOK, resp.StatusCode)

	resp, err = http.Get("http://localhost:9091" + opts.RoutePrefix + "/version")

	testutil.Ok(t, err)
	testutil.Equals(t, http.StatusOK, resp.StatusCode)

	resp, err = http.Post("http://localhost:9091"+opts.RoutePrefix+"/api/v2/admin/tsdb/snapshot", "", strings.NewReader(""))

	testutil.Ok(t, err)
	testutil.Equals(t, http.StatusOK, resp.StatusCode)

	resp, err = http.Post("http://localhost:9091"+opts.RoutePrefix+"/api/v2/admin/tsdb/delete_series", "", strings.NewReader("{}"))

	testutil.Ok(t, err)
	testutil.Equals(t, http.StatusOK, resp.StatusCode)
}

func TestDebugHandler(t *testing.T) {
	for _, tc := range []struct {
		prefix, url string
		code        int
	}{
		{"/", "/debug/pprof/cmdline", 200},
		{"/foo", "/foo/debug/pprof/cmdline", 200},

		{"/", "/debug/pprof/goroutine", 200},
		{"/foo", "/foo/debug/pprof/goroutine", 200},

		{"/", "/debug/pprof/foo", 404},
		{"/foo", "/bar/debug/pprof/goroutine", 404},
	} {
		opts := &Options{
			RoutePrefix: tc.prefix,
		}
		handler := New(nil, opts)
		handler.Ready()

		w := httptest.NewRecorder()

		req, err := http.NewRequest("GET", tc.url, nil)

		testutil.Ok(t, err)

		handler.router.ServeHTTP(w, req)

		testutil.Equals(t, tc.code, w.Code)
	}
}
