/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package source

import (
	"context"
	"sync"

	"sigs.k8s.io/external-dns/endpoint"
)

// multiSource is a Source that merges the endpoints of its nested Sources.
type multiSource struct {
	children       []Source
	defaultTargets []string
}

// Endpoints collects endpoints of all nested Sources and returns them in a single slice.
func (ms *multiSource) Endpoints(ctx context.Context) ([]*endpoint.Endpoint, error) {
	result := []*endpoint.Endpoint{}
	lock := sync.Mutex{}
	wg := sync.WaitGroup{}
	sem := make(chan struct{}, 8)
	var err error
	for _, s := range ms.children {
		sem <- struct{}{}
		wg.Add(1)
		go func(s Source) {
			defer wg.Done()
			defer func() {
				<-sem
			}()
			endpoints, err2 := s.Endpoints(ctx)
			lock.Lock()
			defer lock.Unlock()
			if err2 != nil {
				err = err2
				return
			}
			if len(ms.defaultTargets) > 0 {
				for i := range endpoints {
					eps := endpointsForHostname(endpoints[i].DNSName, ms.defaultTargets, endpoints[i].RecordTTL, endpoints[i].ProviderSpecific, endpoints[i].SetIdentifier, "")
					for _, ep := range eps {
						ep.Labels = endpoints[i].Labels
					}
					result = append(result, eps...)
				}
			} else {
				result = append(result, endpoints...)
			}
		}(s)

	}
	wg.Wait()
	return result, err
}

func (ms *multiSource) AddEventHandler(ctx context.Context, handler func()) {
	for _, s := range ms.children {
		s.AddEventHandler(ctx, handler)
	}
}

// NewMultiSource creates a new multiSource.
func NewMultiSource(children []Source, defaultTargets []string) Source {
	return &multiSource{children: children, defaultTargets: defaultTargets}
}
