package azuremonitorreceiver

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/monitor/armmonitor"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
)

type WrapperInterface interface {
	Init(tenantId, clientId, clientSecret, subscriptionId string) error
	GetResourcesPager(*armresources.ClientListOptions) ResourcesPagerInterface
	GetMetricsDefinitionsPager(string) MetricsDefinitionsPagerInterface
	GetMetricsValues(ctx context.Context, resourceId string, options *armmonitor.MetricsClientListOptions) (armmonitor.MetricsClientListResponse, error)
}

type Wrapper struct {
	cred                     azcore.TokenCredential
	clientResources          *armresources.Client
	clientMetricsDefinitions *armmonitor.MetricDefinitionsClient
	clientMetricsValues      *armmonitor.MetricsClient
}

func (w *Wrapper) Init(tenantId, clientId, clientSecret, subscriptionId string) error {
	var err error
	w.cred, err = azidentity.NewClientSecretCredential(tenantId, clientId, clientSecret, nil)
	if err != nil {
		return err
	}

	w.clientResources, _ = armresources.NewClient(subscriptionId, w.cred, nil)
	w.clientMetricsDefinitions, _ = armmonitor.NewMetricDefinitionsClient(w.cred, nil)
	w.clientMetricsValues, _ = armmonitor.NewMetricsClient(w.cred, nil)
	return nil
}

func (w *Wrapper) GetResourcesPager(options *armresources.ClientListOptions) ResourcesPagerInterface {
	pager := w.clientResources.NewListPager(options)
	return &ResourcesPager{pager: pager}
}

func (w *Wrapper) GetMetricsDefinitionsPager(resourceId string) MetricsDefinitionsPagerInterface {
	pager := w.clientMetricsDefinitions.NewListPager(resourceId, nil)
	return &MetricsDefinitionsPager{pager: pager}
}

func (w *Wrapper) GetMetricsValues(ctx context.Context, resourceId string, options *armmonitor.MetricsClientListOptions) (armmonitor.MetricsClientListResponse, error) {
	result, err := w.clientMetricsValues.List(
		ctx,
		resourceId,
		options,
	)
	return result, err
}

type ResourcesPagerInterface interface {
	More() bool
	NextPage(ctx context.Context) (armresources.ClientListResponse, error)
}

type ResourcesPager struct {
	pager *runtime.Pager[armresources.ClientListResponse]
}

func (rp *ResourcesPager) More() bool {
	return rp.pager.More()
}

func (rp *ResourcesPager) NextPage(ctx context.Context) (armresources.ClientListResponse, error) {
	return rp.pager.NextPage(ctx)
}

type MetricsDefinitionsPagerInterface interface {
	More() bool
	NextPage(ctx context.Context) (armmonitor.MetricDefinitionsClientListResponse, error)
}

type MetricsDefinitionsPager struct {
	pager *runtime.Pager[armmonitor.MetricDefinitionsClientListResponse]
}

func (rp *MetricsDefinitionsPager) More() bool {
	return rp.pager.More()
}

func (rp *MetricsDefinitionsPager) NextPage(ctx context.Context) (armmonitor.MetricDefinitionsClientListResponse, error) {
	return rp.pager.NextPage(ctx)
}
