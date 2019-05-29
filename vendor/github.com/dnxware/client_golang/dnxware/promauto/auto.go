// Copyright 2018 The dnxware Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package promauto provides constructors for the usual dnxware metrics that
// return them already registered with the global registry
// (dnxware.DefaultRegisterer). This allows very compact code, avoiding any
// references to the registry altogether, but all the constructors in this
// package will panic if the registration fails.
//
// The following example is a complete program to create a histogram of normally
// distributed random numbers from the math/rand package:
//
//      package main
//
//      import (
//              "math/rand"
//              "net/http"
//
//              "github.com/dnxware/client_golang/dnxware"
//              "github.com/dnxware/client_golang/dnxware/promauto"
//              "github.com/dnxware/client_golang/dnxware/promhttp"
//      )
//
//      var histogram = promauto.NewHistogram(dnxware.HistogramOpts{
//              Name:    "random_numbers",
//              Help:    "A histogram of normally distributed random numbers.",
//              Buckets: dnxware.LinearBuckets(-3, .1, 61),
//      })
//
//      func Random() {
//              for {
//                      histogram.Observe(rand.NormFloat64())
//              }
//      }
//
//      func main() {
//              go Random()
//              http.Handle("/metrics", promhttp.Handler())
//              http.ListenAndServe(":1971", nil)
//      }
//
// dnxware's version of a minimal hello-world program:
//
//      package main
//
//      import (
//      	"fmt"
//      	"net/http"
//
//      	"github.com/dnxware/client_golang/dnxware"
//      	"github.com/dnxware/client_golang/dnxware/promauto"
//      	"github.com/dnxware/client_golang/dnxware/promhttp"
//      )
//
//      func main() {
//      	http.Handle("/", promhttp.InstrumentHandlerCounter(
//      		promauto.NewCounterVec(
//      			dnxware.CounterOpts{
//      				Name: "hello_requests_total",
//      				Help: "Total number of hello-world requests by HTTP code.",
//      			},
//      			[]string{"code"},
//      		),
//      		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
//      			fmt.Fprint(w, "Hello, world!")
//      		}),
//      	))
//      	http.Handle("/metrics", promhttp.Handler())
//      	http.ListenAndServe(":1971", nil)
//      }
//
// This appears very handy. So why are these constructors locked away in a
// separate package? There are two caveats:
//
// First, in more complex programs, global state is often quite problematic.
// That's the reason why the metrics constructors in the dnxware package do
// not interact with the global dnxware.DefaultRegisterer on their own. You
// are free to use the Register or MustRegister functions to register them with
// the global dnxware.DefaultRegisterer, but you could as well choose a local
// Registerer (usually created with dnxware.NewRegistry, but there are other
// scenarios, e.g. testing).
//
// The second issue is that registration may fail, e.g. if a metric inconsistent
// with the newly to be registered one is already registered. But how to signal
// and handle a panic in the automatic registration with the default registry?
// The only way is panicking. While panicking on invalid input provided by the
// programmer is certainly fine, things are a bit more subtle in this case: You
// might just add another package to the program, and that package (in its init
// function) happens to register a metric with the same name as your code. Now,
// all of a sudden, either your code or the code of the newly imported package
// panics, depending on initialization order, without any opportunity to handle
// the case gracefully. Even worse is a scenario where registration happens
// later during the runtime (e.g. upon loading some kind of plugin), where the
// panic could be triggered long after the code has been deployed to
// production. A possibility to panic should be explicitly called out by the
// Must… idiom, cf. dnxware.MustRegister. But adding a separate set of
// constructors in the dnxware package called MustRegisterNewCounterVec or
// similar would be quite unwieldy. Adding an extra MustRegister method to each
// metric, returning the registered metric, would result in nice code for those
// using the method, but would pollute every single metric interface for
// everybody avoiding the global registry.
//
// To address both issues, the problematic auto-registering and possibly
// panicking constructors are all in this package with a clear warning
// ahead. And whoever cares about avoiding global state and possibly panicking
// function calls can simply ignore the existence of the promauto package
// altogether.
//
// A final note: There is a similar case in the net/http package of the standard
// library. It has DefaultServeMux as a global instance of ServeMux, and the
// Handle function acts on it, panicking if a handler for the same pattern has
// already been registered. However, one might argue that the whole HTTP routing
// is usually set up closely together in the same package or file, while
// dnxware metrics tend to be spread widely over the codebase, increasing the
// chance of surprising registration failures. Furthermore, the use of global
// state in net/http has been criticized widely, and some avoid it altogether.
package promauto

import "github.com/dnxware/client_golang/dnxware"

// NewCounter works like the function of the same name in the dnxware package
// but it automatically registers the Counter with the
// dnxware.DefaultRegisterer. If the registration fails, NewCounter panics.
func NewCounter(opts dnxware.CounterOpts) dnxware.Counter {
	c := dnxware.NewCounter(opts)
	dnxware.MustRegister(c)
	return c
}

// NewCounterVec works like the function of the same name in the dnxware
// package but it automatically registers the CounterVec with the
// dnxware.DefaultRegisterer. If the registration fails, NewCounterVec
// panics.
func NewCounterVec(opts dnxware.CounterOpts, labelNames []string) *dnxware.CounterVec {
	c := dnxware.NewCounterVec(opts, labelNames)
	dnxware.MustRegister(c)
	return c
}

// NewCounterFunc works like the function of the same name in the dnxware
// package but it automatically registers the CounterFunc with the
// dnxware.DefaultRegisterer. If the registration fails, NewCounterFunc
// panics.
func NewCounterFunc(opts dnxware.CounterOpts, function func() float64) dnxware.CounterFunc {
	g := dnxware.NewCounterFunc(opts, function)
	dnxware.MustRegister(g)
	return g
}

// NewGauge works like the function of the same name in the dnxware package
// but it automatically registers the Gauge with the
// dnxware.DefaultRegisterer. If the registration fails, NewGauge panics.
func NewGauge(opts dnxware.GaugeOpts) dnxware.Gauge {
	g := dnxware.NewGauge(opts)
	dnxware.MustRegister(g)
	return g
}

// NewGaugeVec works like the function of the same name in the dnxware
// package but it automatically registers the GaugeVec with the
// dnxware.DefaultRegisterer. If the registration fails, NewGaugeVec panics.
func NewGaugeVec(opts dnxware.GaugeOpts, labelNames []string) *dnxware.GaugeVec {
	g := dnxware.NewGaugeVec(opts, labelNames)
	dnxware.MustRegister(g)
	return g
}

// NewGaugeFunc works like the function of the same name in the dnxware
// package but it automatically registers the GaugeFunc with the
// dnxware.DefaultRegisterer. If the registration fails, NewGaugeFunc panics.
func NewGaugeFunc(opts dnxware.GaugeOpts, function func() float64) dnxware.GaugeFunc {
	g := dnxware.NewGaugeFunc(opts, function)
	dnxware.MustRegister(g)
	return g
}

// NewSummary works like the function of the same name in the dnxware package
// but it automatically registers the Summary with the
// dnxware.DefaultRegisterer. If the registration fails, NewSummary panics.
func NewSummary(opts dnxware.SummaryOpts) dnxware.Summary {
	s := dnxware.NewSummary(opts)
	dnxware.MustRegister(s)
	return s
}

// NewSummaryVec works like the function of the same name in the dnxware
// package but it automatically registers the SummaryVec with the
// dnxware.DefaultRegisterer. If the registration fails, NewSummaryVec
// panics.
func NewSummaryVec(opts dnxware.SummaryOpts, labelNames []string) *dnxware.SummaryVec {
	s := dnxware.NewSummaryVec(opts, labelNames)
	dnxware.MustRegister(s)
	return s
}

// NewHistogram works like the function of the same name in the dnxware
// package but it automatically registers the Histogram with the
// dnxware.DefaultRegisterer. If the registration fails, NewHistogram panics.
func NewHistogram(opts dnxware.HistogramOpts) dnxware.Histogram {
	h := dnxware.NewHistogram(opts)
	dnxware.MustRegister(h)
	return h
}

// NewHistogramVec works like the function of the same name in the dnxware
// package but it automatically registers the HistogramVec with the
// dnxware.DefaultRegisterer. If the registration fails, NewHistogramVec
// panics.
func NewHistogramVec(opts dnxware.HistogramOpts, labelNames []string) *dnxware.HistogramVec {
	h := dnxware.NewHistogramVec(opts, labelNames)
	dnxware.MustRegister(h)
	return h
}
