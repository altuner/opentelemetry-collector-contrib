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
	"github.com/stretchr/testify/require"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/monitor/armmonitor"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azuremonitorreceiver/internal/metadata"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestNewScraper(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)

	scraper := newScraper(cfg, receivertest.NewNopCreateSettings())
	require.Len(t, scraper.resources, 0)
}

type WrapperMock struct {
	WrapperInterface

	resourcesMockData          *ResourcesPagerMock
	metricsDefinitionsMockData map[string]*MetricsDefinitionsPagerMockMock
	metricsValuesMockData      map[string]armmonitor.MetricsClientListResponse
}

func (wm *WrapperMock) Init(tenantId, clientId, clientSecret, subscriptionId string) error {
	return nil
}

func (wm *WrapperMock) GetResourcesPager(options *armresources.ClientListOptions) ResourcesPagerInterface {
	return wm.resourcesMockData
}

func (wm *WrapperMock) GetMetricsDefinitionsPager(resourceId string) MetricsDefinitionsPagerInterface {
	return wm.metricsDefinitionsMockData[resourceId]
}

func (wm *WrapperMock) GetMetricsValues(ctx context.Context, resourceId string, options *armmonitor.MetricsClientListOptions) (armmonitor.MetricsClientListResponse, error) {
	return wm.metricsValuesMockData[resourceId], nil
}

type ResourcesPagerMock struct {
	current int
	pages   []armresources.ClientListResponse
}

func (rpm *ResourcesPagerMock) More() bool {
	return rpm.current < len(rpm.pages)
}

func (rpm *ResourcesPagerMock) NextPage(ctx context.Context) (armresources.ClientListResponse, error) {
	page := rpm.pages[rpm.current]
	rpm.current++
	return page, nil
}

type MetricsDefinitionsPagerMockMock struct {
	current int
	pages   []armmonitor.MetricDefinitionsClientListResponse
}

func (mdpm *MetricsDefinitionsPagerMockMock) More() bool {
	return mdpm.current < len(mdpm.pages)
}

func (mdpm *MetricsDefinitionsPagerMockMock) NextPage(ctx context.Context) (armmonitor.MetricDefinitionsClientListResponse, error) {
	page := mdpm.pages[mdpm.current]
	mdpm.current++
	return page, nil
}

func TestAzureScraperStart(t *testing.T) {
	type fields struct {
		cfg *Config
	}
	type args struct {
		ctx  context.Context
		host component.Host
	}

	cfg := createDefaultConfig().(*Config)

	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name: "1st",
			fields: fields{
				cfg: cfg,
			},
			args: args{
				ctx:  context.Background(),
				host: componenttest.NewNopHost(),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &azureScraper{
				cfg:     tt.fields.cfg,
				wrapper: &WrapperMock{},
			}

			if err := s.start(tt.args.ctx, tt.args.host); (err != nil) != tt.wantErr {
				t.Errorf("azureScraper.start() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAzureScraperScrape(t *testing.T) {
	type fields struct {
		cfg *Config
	}
	type args struct {
		ctx context.Context
	}
	cfg := createDefaultConfig().(*Config)
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "",
			fields: fields{
				cfg: cfg,
			},
			args: args{
				ctx: context.Background(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := receivertest.NewNopCreateSettings()
			w := &WrapperMock{}
			w.resourcesMockData = getResourcesMockData()
			w.metricsDefinitionsMockData = GetMetricsDefinitionsMockData()
			w.metricsValuesMockData = GetMetricsValuesMockData()

			s := &azureScraper{
				cfg:     tt.fields.cfg,
				wrapper: w,
				mb:      metadata.NewMetricsBuilder(metadata.DefaultMetricsBuilderConfig(), settings),
			}
			s.resources = map[string]*azureResource{}
			metrics, err := s.scrape(tt.args.ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("azureScraper.scrape() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			require.EqualValues(t, 11, metrics.MetricCount(), "Scraper should have return expected number of metrics")
		})
	}
}

func getResourcesMockData() *ResourcesPagerMock {
	id1, id2, id3 := "resourceId1", "resourceId2", "resourceId3"
	return &ResourcesPagerMock{
		current: 0,
		pages: []armresources.ClientListResponse{
			armresources.ClientListResponse{
				ResourceListResult: armresources.ResourceListResult{
					Value: []*armresources.GenericResourceExpanded{
						&armresources.GenericResourceExpanded{
							ID: &id1,
						},
					},
				},
			},
			armresources.ClientListResponse{
				ResourceListResult: armresources.ResourceListResult{
					Value: []*armresources.GenericResourceExpanded{
						&armresources.GenericResourceExpanded{
							ID: &id2,
						},
					},
				},
			},
			armresources.ClientListResponse{
				ResourceListResult: armresources.ResourceListResult{
					Value: []*armresources.GenericResourceExpanded{
						&armresources.GenericResourceExpanded{
							ID: &id3,
						},
					},
				},
			},
		},
	}
}

func GetMetricsDefinitionsMockData() map[string]*MetricsDefinitionsPagerMockMock {
	name1 := "metric1"
	timeGrain := "PT1M"
	return map[string]*MetricsDefinitionsPagerMockMock{
		"resourceId1": &MetricsDefinitionsPagerMockMock{
			current: 0,
			pages: []armmonitor.MetricDefinitionsClientListResponse{
				armmonitor.MetricDefinitionsClientListResponse{
					MetricDefinitionCollection: armmonitor.MetricDefinitionCollection{
						Value: []*armmonitor.MetricDefinition{
							&armmonitor.MetricDefinition{
								Name: &armmonitor.LocalizableString{
									Value: &name1,
								},
								MetricAvailabilities: []*armmonitor.MetricAvailability{
									{
										TimeGrain: &timeGrain,
									},
								},
							},
						},
					},
				},
			},
		},
		"resourceId2": &MetricsDefinitionsPagerMockMock{
			current: 0,
			pages: []armmonitor.MetricDefinitionsClientListResponse{
				armmonitor.MetricDefinitionsClientListResponse{
					MetricDefinitionCollection: armmonitor.MetricDefinitionCollection{
						Value: []*armmonitor.MetricDefinition{
							&armmonitor.MetricDefinition{
								Name: &armmonitor.LocalizableString{
									Value: &name1,
								},
								MetricAvailabilities: []*armmonitor.MetricAvailability{
									{
										TimeGrain: &timeGrain,
									},
								},
							},
						},
					},
				},
			},
		},
		"resourceId3": &MetricsDefinitionsPagerMockMock{
			current: 0,
			pages: []armmonitor.MetricDefinitionsClientListResponse{
				armmonitor.MetricDefinitionsClientListResponse{
					MetricDefinitionCollection: armmonitor.MetricDefinitionCollection{
						Value: []*armmonitor.MetricDefinition{
							&armmonitor.MetricDefinition{
								Name: &armmonitor.LocalizableString{
									Value: &name1,
								},
								MetricAvailabilities: []*armmonitor.MetricAvailability{
									{
										TimeGrain: &timeGrain,
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func GetMetricsValuesMockData() map[string]armmonitor.MetricsClientListResponse {
	name1 := "metric1"
	var unit1 armmonitor.MetricUnit = "unit1"
	var value1 float64 = 1
	return map[string]armmonitor.MetricsClientListResponse{
		"resourceId1": armmonitor.MetricsClientListResponse{
			Response: armmonitor.Response{
				Value: []*armmonitor.Metric{
					&armmonitor.Metric{
						Name: &armmonitor.LocalizableString{
							Value: &name1,
						},
						Unit: &unit1,
						Timeseries: []*armmonitor.TimeSeriesElement{
							&armmonitor.TimeSeriesElement{
								Data: []*armmonitor.MetricValue{
									&armmonitor.MetricValue{
										Average: &value1,
										Count:   &value1,
										Maximum: &value1,
										Minimum: &value1,
										Total:   &value1,
									},
								},
							},
						},
					},
				},
			},
		},
		"resourceId2": armmonitor.MetricsClientListResponse{
			Response: armmonitor.Response{
				Value: []*armmonitor.Metric{
					&armmonitor.Metric{
						Name: &armmonitor.LocalizableString{
							Value: &name1,
						},
						Unit: &unit1,
						Timeseries: []*armmonitor.TimeSeriesElement{
							&armmonitor.TimeSeriesElement{
								Data: []*armmonitor.MetricValue{
									&armmonitor.MetricValue{
										Average: &value1,
										Count:   &value1,
										Maximum: &value1,
										Minimum: &value1,
										Total:   &value1,
									},
								},
							},
						},
					},
				},
			},
		},
		"resourceId3": armmonitor.MetricsClientListResponse{
			Response: armmonitor.Response{
				Value: []*armmonitor.Metric{
					&armmonitor.Metric{
						Name: &armmonitor.LocalizableString{
							Value: &name1,
						},
						Unit: &unit1,
						Timeseries: []*armmonitor.TimeSeriesElement{
							&armmonitor.TimeSeriesElement{
								Data: []*armmonitor.MetricValue{
									&armmonitor.MetricValue{
										Count: &value1,
									},
								},
							},
						},
					},
				},
			},
		},
	}
}
