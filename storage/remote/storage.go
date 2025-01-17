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

package remote

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"sync"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"

	"github.com/dnxware/client_golang/dnxware"
	"github.com/dnxware/common/model"
	"github.com/dnxware/dnxware/config"
	"github.com/dnxware/dnxware/pkg/labels"
	"github.com/dnxware/dnxware/pkg/logging"
	"github.com/dnxware/dnxware/storage"
)

// startTimeCallback is a callback func that return the oldest timestamp stored in a storage.
type startTimeCallback func() (int64, error)

// Storage represents all the remote read and write endpoints.  It implements
// storage.Storage.
type Storage struct {
	logger log.Logger
	mtx    sync.Mutex

	configHash [16]byte

	// For writes
	walDir        string
	queues        []*QueueManager
	samplesIn     *ewmaRate
	flushDeadline time.Duration

	// For reads
	queryables             []storage.Queryable
	localStartTimeCallback startTimeCallback
}

// NewStorage returns a remote.Storage.
func NewStorage(l log.Logger, reg dnxware.Registerer, stCallback startTimeCallback, walDir string, flushDeadline time.Duration) *Storage {
	if l == nil {
		l = log.NewNopLogger()
	}
	s := &Storage{
		logger:                 logging.Dedupe(l, 1*time.Minute),
		localStartTimeCallback: stCallback,
		flushDeadline:          flushDeadline,
		samplesIn:              newEWMARate(ewmaWeight, shardUpdateDuration),
		walDir:                 walDir,
	}
	go s.run()
	return s
}

func (s *Storage) run() {
	ticker := time.NewTicker(shardUpdateDuration)
	defer ticker.Stop()
	for range ticker.C {
		s.samplesIn.tick()
	}
}

// ApplyConfig updates the state as the new config requires.
func (s *Storage) ApplyConfig(conf *config.Config) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	if err := s.applyRemoteWriteConfig(conf); err != nil {
		return err
	}

	// Update read clients
	queryables := make([]storage.Queryable, 0, len(conf.RemoteReadConfigs))
	for i, rrConf := range conf.RemoteReadConfigs {
		c, err := NewClient(i, &ClientConfig{
			URL:              rrConf.URL,
			Timeout:          rrConf.RemoteTimeout,
			HTTPClientConfig: rrConf.HTTPClientConfig,
		})
		if err != nil {
			return err
		}

		q := QueryableClient(c)
		q = ExternalLabelsHandler(q, conf.GlobalConfig.ExternalLabels)
		if len(rrConf.RequiredMatchers) > 0 {
			q = RequiredMatchersFilter(q, labelsToEqualityMatchers(rrConf.RequiredMatchers))
		}
		if !rrConf.ReadRecent {
			q = PreferLocalStorageFilter(q, s.localStartTimeCallback)
		}
		queryables = append(queryables, q)
	}
	s.queryables = queryables

	return nil
}

// applyRemoteWriteConfig applies the remote write config only if the config has changed.
// The caller must hold the lock on s.mtx.
func (s *Storage) applyRemoteWriteConfig(conf *config.Config) error {
	// Remote write queues only need to change if the remote write config or
	// external labels change. Hash these together and only reload if the hash
	// changes.
	cfgBytes, err := json.Marshal(conf.RemoteWriteConfigs)
	if err != nil {
		return err
	}
	externalLabelBytes, err := json.Marshal(conf.GlobalConfig.ExternalLabels)
	if err != nil {
		return err
	}

	hash := md5.Sum(append(cfgBytes, externalLabelBytes...))
	if hash == s.configHash {
		level.Debug(s.logger).Log("msg", "remote write config has not changed, no need to restart QueueManagers")
		return nil
	}

	s.configHash = hash

	// Update write queues
	newQueues := []*QueueManager{}
	// TODO: we should only stop & recreate queues which have changes,
	// as this can be quite disruptive.
	for i, rwConf := range conf.RemoteWriteConfigs {
		c, err := NewClient(i, &ClientConfig{
			URL:              rwConf.URL,
			Timeout:          rwConf.RemoteTimeout,
			HTTPClientConfig: rwConf.HTTPClientConfig,
		})
		if err != nil {
			return err
		}
		newQueues = append(newQueues, NewQueueManager(
			s.logger,
			s.walDir,
			s.samplesIn,
			rwConf.QueueConfig,
			conf.GlobalConfig.ExternalLabels,
			rwConf.WriteRelabelConfigs,
			c,
			s.flushDeadline,
		))
	}

	for _, q := range s.queues {
		q.Stop()
	}

	s.queues = newQueues
	for _, q := range s.queues {
		q.Start()
	}

	return nil
}

// StartTime implements the Storage interface.
func (s *Storage) StartTime() (int64, error) {
	return int64(model.Latest), nil
}

// Querier returns a storage.MergeQuerier combining the remote client queriers
// of each configured remote read endpoint.
func (s *Storage) Querier(ctx context.Context, mint, maxt int64) (storage.Querier, error) {
	s.mtx.Lock()
	queryables := s.queryables
	s.mtx.Unlock()

	queriers := make([]storage.Querier, 0, len(queryables))
	for _, queryable := range queryables {
		q, err := queryable.Querier(ctx, mint, maxt)
		if err != nil {
			return nil, err
		}
		queriers = append(queriers, q)
	}
	return storage.NewMergeQuerier(nil, queriers), nil
}

// Close the background processing of the storage queues.
func (s *Storage) Close() error {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	for _, q := range s.queues {
		q.Stop()
	}

	return nil
}

func labelsToEqualityMatchers(ls model.LabelSet) []*labels.Matcher {
	ms := make([]*labels.Matcher, 0, len(ls))
	for k, v := range ls {
		ms = append(ms, &labels.Matcher{
			Type:  labels.MatchEqual,
			Name:  string(k),
			Value: string(v),
		})
	}
	return ms
}
