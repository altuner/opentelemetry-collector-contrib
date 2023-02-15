// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package azuremonitorreceiver

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/monitor/armmonitor"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"

	"github.com/altuner/opentelemetry-collector-contrib/receiver/azuremonitorreceiver/internal/metadata"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
)

var (
	errNotAuthorized = errors.New("Scraper isn't authorized")
	timeGrains       = map[string]int64{
		"PT1M":  60,
		"PT5M":  300,
		"PT15M": 900,
		"PT30M": 1800,
		"PT1H":  3600,
		"PT6H":  21600,
		"PT12H": 43200,
		"P1D":   86400,
	}
	aggregations = []string{
		"Average",
		"Count",
		"Maximum",
		"Minimum",
		"Total",
	}
)

type azureResource struct {
	metricsByGrains           map[string]*azureResourceMetrics
	metricsDefinitionsUpdated int64
}

type azureResourceMetrics struct {
	metrics              []string
	metricsValuesUpdated int64
}

type void struct{}

func newScraper(conf *Config, settings receiver.CreateSettings) *azureScraper {
	return &azureScraper{
		cfg:                             conf,
		settings:                        settings.TelemetrySettings,
		mb:                              metadata.NewMetricsBuilder(settings),
		azIdCredentialsClientFunc:       azidentity.NewClientSecretCredential,
		armClientFunc:                   armresources.NewClient,
		armMonitorDefinitionsClientFunc: armmonitor.NewMetricDefinitionsClient,
		armMonitorMetricsClientFunc:     armmonitor.NewMetricsClient,
	}
}

type azureScraper struct {
	cred                            azcore.TokenCredential
	clientResources                 *armresources.Client
	clientMetricsDefinitions        *armmonitor.MetricDefinitionsClient
	clientMetricsValues             *armmonitor.MetricsClient
	cfg                             *Config
	settings                        component.TelemetrySettings
	resources                       map[string]*azureResource
	resourcesUpdated                int64
	mb                              *metadata.MetricsBuilder
	azIdCredentialsClientFunc       func(string, string, string, *azidentity.ClientSecretCredentialOptions) (*azidentity.ClientSecretCredential, error)
	armClientFunc                   func(string, azcore.TokenCredential, *arm.ClientOptions) (*armresources.Client, error)
	armMonitorDefinitionsClientFunc func(azcore.TokenCredential, *arm.ClientOptions) (*armmonitor.MetricDefinitionsClient, error)
	armMonitorMetricsClientFunc     func(azcore.TokenCredential, *arm.ClientOptions) (*armmonitor.MetricsClient, error)
}

func (s *azureScraper) start(ctx context.Context, host component.Host) (err error) {
	s.cred, err = s.azIdCredentialsClientFunc(s.cfg.TenantId, s.cfg.ClientId, s.cfg.ClientSecret, nil)
	if err != nil {
		log.Println("Authentication failure", err)
		return
	}

	s.clientResources, _ = s.armClientFunc(s.cfg.SubscriptionId, s.cred, nil)

	s.clientMetricsDefinitions, err = s.armMonitorDefinitionsClientFunc(s.cred, nil)

	s.clientMetricsValues, _ = s.armMonitorMetricsClientFunc(s.cred, nil)

	s.resources = map[string]*azureResource{}

	return
}

func (s *azureScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	if s.cred == nil {
		return pmetric.NewMetrics(), errNotAuthorized
	}

	s.getResources(ctx)

	resourcesIdsWithDefinitions := make(chan string)

	go func() {
		defer close(resourcesIdsWithDefinitions)
		for resourceId, _ := range s.resources {
			s.getResourceMetricsDefinitions(ctx, resourceId)
			resourcesIdsWithDefinitions <- resourceId
		}
	}()

	var wg sync.WaitGroup

	for resourcesIdsWithDefinitions != nil {
		select {
		case resourceId, ok := <-resourcesIdsWithDefinitions:
			if !ok {
				resourcesIdsWithDefinitions = nil
				break
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				s.getResourceMetricsValues(ctx, resourceId)
			}()
		}
	}

	wg.Wait()

	return s.mb.Emit(), nil
}

func (s *azureScraper) getResources(ctx context.Context) {

	if time.Now().UTC().Unix() < (s.resourcesUpdated + s.cfg.CacheResources) {
		return
	}

	existingResources := map[string]void{}
	for id, _ := range s.resources {
		existingResources[id] = void{}
	}

	// TODO: switch to parsing services from https://learn.microsoft.com/en-us/azure/azure-monitor/essentials/metrics-supported
	resourcesFilter := strings.Join(monitorServices, "' or resourceType eq  '")
	typeFilter := fmt.Sprintf("resourceType eq '%s'", resourcesFilter)

	opts := armresources.ClientListOptions{
		Filter: &typeFilter,
	}

	pager := s.clientResources.NewListPager(&opts)

	for pager.More() {
		nextResult, err := pager.NextPage(ctx)
		if err != nil {
			log.Println("failed to get page data", err)
		}
		for _, resource := range nextResult.Value {

			if _, ok := s.resources[*resource.ID]; !ok {
				s.resources[*resource.ID] = &azureResource{}
			}

			if _, ok := existingResources[*resource.ID]; ok {
				delete(existingResources, *resource.ID)
			}
		}
	}

	if len(existingResources) > 0 {
		for idToDelete, _ := range existingResources {
			if _, ok := s.resources[idToDelete]; ok {
				delete(s.resources, idToDelete)
			}
		}
	}

	s.resourcesUpdated = time.Now().UTC().Unix()
}

func (s *azureScraper) getResourceMetricsDefinitions(ctx context.Context, resourceId string) {

	if time.Now().UTC().Unix() < (s.resources[resourceId].metricsDefinitionsUpdated + s.cfg.CacheResourcesDefinitions) {
		return
	}
	res := s.resources[resourceId]
	res.metricsByGrains = map[string]*azureResourceMetrics{}

	pager := s.clientMetricsDefinitions.NewListPager(resourceId, nil)
	for pager.More() {
		nextResult, err := pager.NextPage(ctx)
		if err != nil {
			log.Println("failed to get page data", err)
		}
		for _, v := range nextResult.Value {

			timeGrain := *v.MetricAvailabilities[0].TimeGrain
			name := *v.Name.Value

			if _, ok := res.metricsByGrains[timeGrain]; ok {
				res.metricsByGrains[timeGrain].metrics = append(res.metricsByGrains[timeGrain].metrics, name)
			} else {
				res.metricsByGrains[timeGrain] = &azureResourceMetrics{metrics: []string{name}}
			}
		}
	}
	res.metricsDefinitionsUpdated = time.Now().UTC().Unix()
}

func (s *azureScraper) getResourceMetricsValues(ctx context.Context, resourceId string) {

	res := s.resources[resourceId]

	for timeGrain, metricsByGrain := range res.metricsByGrains {

		if time.Now().UTC().Unix() < (metricsByGrain.metricsValuesUpdated + timeGrains[timeGrain]) {
			continue
		}
		metricsByGrain.metricsValuesUpdated = time.Now().UTC().Unix()

		max, i := s.cfg.MaximumNumberOfMetricsInACall, 0

		for i < len(metricsByGrain.metrics) {

			end := i + max
			if end > len(metricsByGrain.metrics) {
				end = len(metricsByGrain.metrics)
			}

			resType := strings.Join(metricsByGrain.metrics[i:end], ",")
			i = end

			opts := armmonitor.MetricsClientListOptions{
				Metricnames: &resType,
				Interval:    to.Ptr(timeGrain),
				Timespan:    to.Ptr(timeGrain),
				Aggregation: to.Ptr(strings.Join(aggregations, ",")),
			}

			result, err := s.clientMetricsValues.List(
				ctx,
				resourceId,
				&opts,
			)
			if err != nil {
				log.Println("failed to get metrics data", err)
				return
			}

			for _, metric := range result.Value {

				for _, timeserie := range metric.Timeseries {
					if timeserie.Data != nil {
						for _, timeserieData := range timeserie.Data {

							ts := pcommon.NewTimestampFromTime(time.Now())
							if timeserieData.Average != nil {
								s.mb.AddDataPoint(resourceId, *metric.Name.Value, "Average", string(*metric.Unit), ts, *timeserieData.Average)
							}
							if timeserieData.Count != nil {
								s.mb.AddDataPoint(resourceId, *metric.Name.Value, "Count", string(*metric.Unit), ts, *timeserieData.Count)
							}
							if timeserieData.Maximum != nil {
								s.mb.AddDataPoint(resourceId, *metric.Name.Value, "Maximum", string(*metric.Unit), ts, *timeserieData.Maximum)
							}
							if timeserieData.Minimum != nil {
								s.mb.AddDataPoint(resourceId, *metric.Name.Value, "Minimum", string(*metric.Unit), ts, *timeserieData.Minimum)
							}
							if timeserieData.Total != nil {
								s.mb.AddDataPoint(resourceId, *metric.Name.Value, "Total", string(*metric.Unit), ts, *timeserieData.Total)
							}
						}
					}
				}
			}
		}
	}
}
