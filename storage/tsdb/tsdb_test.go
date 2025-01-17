// Copyright 2017 The dnxware Authors
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

package tsdb_test

import (
	"testing"

	dto "github.com/dnxware/client_model/go"
	"github.com/dnxware/common/model"
	"github.com/dnxware/dnxware/pkg/labels"
	"github.com/dnxware/dnxware/storage/tsdb"
	"github.com/dnxware/dnxware/util/testutil"
)

func TestMetrics(t *testing.T) {
	db := testutil.NewStorage(t)
	defer db.Close()

	metrics := &dto.Metric{}
	startTime := *tsdb.StartTime
	headMinTime := *tsdb.HeadMinTime
	headMaxTime := *tsdb.HeadMaxTime

	// Check initial values.
	testutil.Ok(t, startTime.Write(metrics))
	testutil.Equals(t, float64(model.Latest)/1000, metrics.Gauge.GetValue())

	testutil.Ok(t, headMinTime.Write(metrics))
	testutil.Equals(t, float64(model.Latest)/1000, metrics.Gauge.GetValue())

	testutil.Ok(t, headMaxTime.Write(metrics))
	testutil.Equals(t, float64(model.Earliest)/1000, metrics.Gauge.GetValue())

	app, err := db.Appender()
	testutil.Ok(t, err)

	app.Add(labels.FromStrings(model.MetricNameLabel, "a"), 1, 1)
	app.Add(labels.FromStrings(model.MetricNameLabel, "a"), 2, 1)
	app.Add(labels.FromStrings(model.MetricNameLabel, "a"), 3, 1)
	testutil.Ok(t, app.Commit())

	// Check after adding some samples.
	testutil.Ok(t, startTime.Write(metrics))
	testutil.Equals(t, 0.001, metrics.Gauge.GetValue())

	testutil.Ok(t, headMinTime.Write(metrics))
	testutil.Equals(t, 0.001, metrics.Gauge.GetValue())

	testutil.Ok(t, headMaxTime.Write(metrics))
	testutil.Equals(t, 0.003, metrics.Gauge.GetValue())

}
