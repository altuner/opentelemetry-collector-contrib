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
	//"reflect"
	"log"
	"testing"

	// "github.com/Azure/azure-sdk-for-go/sdk/azcore"
	// "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/monitor/armmonitor"
	// "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	// "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azuremonitorreceiver/internal/metadata"

	//"go.opentelemetry.io/collector/pdata/pmetric"

	//"go.opentelemetry.io/collector/receiver"
	"github.com/stretchr/testify/require"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/monitor/armmonitor"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
)

func TestNewScraper(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)

	scraper := newScraper(cfg, receivertest.NewNopCreateSettings())
	require.Len(t, scraper.resources, 0)
}

func azIdCredentialsClientFuncMock(string, string, string, *azidentity.ClientSecretCredentialOptions) (*azidentity.ClientSecretCredential, error) {
	return &azidentity.ClientSecretCredential{}, nil
}

func armClientFuncMock(string, azcore.TokenCredential, *arm.ClientOptions) (*armresources.Client, error) {
	return &armresources.Client{}, nil
}

func armMonitorDefinitionsClientFuncMock(azcore.TokenCredential, *arm.ClientOptions) (*armmonitor.MetricDefinitionsClient, error) {
	return &armmonitor.MetricDefinitionsClient{}, nil
}

func armMonitorMetricsClientFuncMock(azcore.TokenCredential, *arm.ClientOptions) (*armmonitor.MetricsClient, error) {
	return &armmonitor.MetricsClient{}, nil
}

func TestAzureScraperStart(t *testing.T) {
	type fields struct {
		cfg *Config
	}
	type args struct {
		ctx  context.Context
		host component.Host
	}

	//cfg := createDefaultConfig().(*Config)

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
				cfg: createDefaultConfig().(*Config),
				// cred: "",
				// clientResources: "",
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
			log.Println(tt.fields.cfg, "client-Id", tt.fields.cfg.AzureSettings.ClientId, "!!!!")
			s := &azureScraper{
				cfg:                             tt.fields.cfg,
				azIdCredentialsClientFunc:       azIdCredentialsClientFuncMock,
				armClientFunc:                   armClientFuncMock,
				armMonitorDefinitionsClientFunc: armMonitorDefinitionsClientFuncMock,
				armMonitorMetricsClientFunc:     armMonitorMetricsClientFuncMock,
			}

			if err := s.start(tt.args.ctx, tt.args.host); (err != nil) != tt.wantErr {
				t.Errorf("azureScraper.start() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// func TestAzureScraperScrape(t *testing.T) {
// 	type fields struct {
// 		// cred                     azcore.TokenCredential
// 		// clientResources          *armresources.Client
// 		// clientMetricsDefinitions *armmonitor.MetricDefinitionsClient
// 		// clientMetricsValues      *armmonitor.MetricsClient
// 		cfg *Config
// 		// settings                 component.TelemetrySettings
// 		// resources                map[string]*azureResource
// 		// resourcesUpdated         int64
// 		// mb                       *metadata.MetricsBuilder
// 	}
// 	type args struct {
// 		ctx context.Context
// 	}
// 	tests := []struct {
// 		name    string
// 		fields  fields
// 		args    args
// 		want    pmetric.Metrics
// 		wantErr bool
// 	}{
// 		// TODO: Add test cases.
// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			s := &azureScraper{
// 				// cred:                     tt.fields.cred,
// 				// clientResources:          tt.fields.clientResources,
// 				// clientMetricsDefinitions: tt.fields.clientMetricsDefinitions,
// 				// clientMetricsValues:      tt.fields.clientMetricsValues,
// 				cfg: tt.fields.cfg,
// 				// settings:                 tt.fields.settings,
// 				// resources:                tt.fields.resources,
// 				// resourcesUpdated:         tt.fields.resourcesUpdated,
// 				// mb:                       tt.fields.mb,
// 			}
// 			got, err := s.scrape(tt.args.ctx)
// 			if (err != nil) != tt.wantErr {
// 				s
// 				t.Errorf("azureScraper.scrape() error = %v, wantErr %v", err, tt.wantErr)
// 				return
// 			}
// 			if !reflect.DeepEqual(got, tt.want) {
// 				t.Errorf("azureScraper.scrape() = %v, want %v", got, tt.want)
// 			}
// 		})
// 	}
// }

// func Test_azureScraper_getResources(t *testing.T) {
// 	type fields struct {
// 		cred                     azcore.TokenCredential
// 		clientResources          *armresources.Client
// 		clientMetricsDefinitions *armmonitor.MetricDefinitionsClient
// 		clientMetricsValues      *armmonitor.MetricsClient
// 		cfg                      *Config
// 		settings                 component.TelemetrySettings
// 		resources                map[string]*azureResource
// 		resourcesUpdated         int64
// 		mb                       *metadata.MetricsBuilder
// 	}
// 	type args struct {
// 		ctx context.Context
// 	}
// 	tests := []struct {
// 		name   string
// 		fields fields
// 		args   args
// 	}{
// 		// TODO: Add test cases.
// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			s := &azureScraper{
// 				cred:                     tt.fields.cred,
// 				clientResources:          tt.fields.clientResources,
// 				clientMetricsDefinitions: tt.fields.clientMetricsDefinitions,
// 				clientMetricsValues:      tt.fields.clientMetricsValues,
// 				cfg:                      tt.fields.cfg,
// 				settings:                 tt.fields.settings,
// 				resources:                tt.fields.resources,
// 				resourcesUpdated:         tt.fields.resourcesUpdated,
// 				mb:                       tt.fields.mb,
// 			}
// 			s.getResources(tt.args.ctx)
// 		})
// 	}
// }

// func Test_azureScraper_checkMetricsDefinitionsCache(t *testing.T) {
// 	type fields struct {
// 		cred                     azcore.TokenCredential
// 		clientResources          *armresources.Client
// 		clientMetricsDefinitions *armmonitor.MetricDefinitionsClient
// 		clientMetricsValues      *armmonitor.MetricsClient
// 		cfg                      *Config
// 		settings                 component.TelemetrySettings
// 		resources                map[string]*azureResource
// 		resourcesUpdated         int64
// 		mb                       *metadata.MetricsBuilder
// 	}
// 	type args struct {
// 		resourceId string
// 	}
// 	tests := []struct {
// 		name   string
// 		fields fields
// 		args   args
// 		want   bool
// 	}{
// 		// TODO: Add test cases.
// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			s := &azureScraper{
// 				cred:                     tt.fields.cred,
// 				clientResources:          tt.fields.clientResources,
// 				clientMetricsDefinitions: tt.fields.clientMetricsDefinitions,
// 				clientMetricsValues:      tt.fields.clientMetricsValues,
// 				cfg:                      tt.fields.cfg,
// 				settings:                 tt.fields.settings,
// 				resources:                tt.fields.resources,
// 				resourcesUpdated:         tt.fields.resourcesUpdated,
// 				mb:                       tt.fields.mb,
// 			}
// 			if got := s.checkMetricsDefinitionsCache(tt.args.resourceId); got != tt.want {
// 				t.Errorf("azureScraper.checkMetricsDefinitionsCache() = %v, want %v", got, tt.want)
// 			}
// 		})
// 	}
// }

// func Test_azureScraper_getResourceMetricsDefinitions(t *testing.T) {
// 	type fields struct {
// 		cred                     azcore.TokenCredential
// 		clientResources          *armresources.Client
// 		clientMetricsDefinitions *armmonitor.MetricDefinitionsClient
// 		clientMetricsValues      *armmonitor.MetricsClient
// 		cfg                      *Config
// 		settings                 component.TelemetrySettings
// 		resources                map[string]*azureResource
// 		resourcesUpdated         int64
// 		mb                       *metadata.MetricsBuilder
// 	}
// 	type args struct {
// 		ctx        context.Context
// 		resourceId string
// 	}
// 	tests := []struct {
// 		name   string
// 		fields fields
// 		args   args
// 	}{
// 		// TODO: Add test cases.
// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			s := &azureScraper{
// 				cred:                     tt.fields.cred,
// 				clientResources:          tt.fields.clientResources,
// 				clientMetricsDefinitions: tt.fields.clientMetricsDefinitions,
// 				clientMetricsValues:      tt.fields.clientMetricsValues,
// 				cfg:                      tt.fields.cfg,
// 				settings:                 tt.fields.settings,
// 				resources:                tt.fields.resources,
// 				resourcesUpdated:         tt.fields.resourcesUpdated,
// 				mb:                       tt.fields.mb,
// 			}
// 			s.getResourceMetricsDefinitions(tt.args.ctx, tt.args.resourceId)
// 		})
// 	}
// }

// func Test_azureScraper_getResourceMetricsValues(t *testing.T) {
// 	type fields struct {
// 		cred                     azcore.TokenCredential
// 		clientResources          *armresources.Client
// 		clientMetricsDefinitions *armmonitor.MetricDefinitionsClient
// 		clientMetricsValues      *armmonitor.MetricsClient
// 		cfg                      *Config
// 		settings                 component.TelemetrySettings
// 		resources                map[string]*azureResource
// 		resourcesUpdated         int64
// 		mb                       *metadata.MetricsBuilder
// 	}
// 	type args struct {
// 		ctx        context.Context
// 		resourceId string
// 	}
// 	tests := []struct {
// 		name   string
// 		fields fields
// 		args   args
// 	}{
// 		// TODO: Add test cases.
// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			s := &azureScraper{
// 				cred:                     tt.fields.cred,
// 				clientResources:          tt.fields.clientResources,
// 				clientMetricsDefinitions: tt.fields.clientMetricsDefinitions,
// 				clientMetricsValues:      tt.fields.clientMetricsValues,
// 				cfg:                      tt.fields.cfg,
// 				settings:                 tt.fields.settings,
// 				resources:                tt.fields.resources,
// 				resourcesUpdated:         tt.fields.resourcesUpdated,
// 				mb:                       tt.fields.mb,
// 			}
// 			s.getResourceMetricsValues(tt.args.ctx, tt.args.resourceId)
// 		})
// 	}
// }
