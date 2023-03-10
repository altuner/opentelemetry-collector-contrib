package metadata

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
)

// ResourceAttributeSettings provides common settings for a particular metric.
type ResourceAttributeSettings struct {
	Enabled bool `mapstructure:"enabled"`

	enabledProvidedByUser bool
}

func (ras *ResourceAttributeSettings) Unmarshal(parser *confmap.Conf) error {
	if parser == nil {
		return nil
	}
	err := parser.Unmarshal(ras, confmap.WithErrorUnused())
	if err != nil {
		return err
	}
	ras.enabledProvidedByUser = parser.IsSet("enabled")
	return nil
}

// ResourceAttributesSettings provides settings for azuremonitorreceiver metrics.
type ResourceAttributesSettings struct {
	AzureMonitorSubscriptionID ResourceAttributeSettings `mapstructure:"azuremonitor.subscription_id"`
	AzureMonitorTenantID       ResourceAttributeSettings `mapstructure:"azuremonitor.tenant_id"`
}

func DefaultResourceAttributesSettings() ResourceAttributesSettings {
	return ResourceAttributesSettings{
		AzureMonitorSubscriptionID: ResourceAttributeSettings{
			Enabled: true,
		},
		AzureMonitorTenantID: ResourceAttributeSettings{
			Enabled: true,
		},
	}
}

func NewMetricsBuilder(mbc MetricsBuilderConfig, settings receiver.CreateSettings, options ...metricBuilderOption) *MetricsBuilder {
	mb := &MetricsBuilder{
		startTime:                  pcommon.NewTimestampFromTime(time.Now()),
		metricsBuffer:              pmetric.NewMetrics(),
		buildInfo:                  settings.BuildInfo,
		resourceAttributesSettings: mbc.ResourceAttributes,
		metrics:                    map[string]*metricAzureAbstract{},
	}
	for _, op := range options {
		op(mb)
	}
	return mb
}

func DefaultMetricsBuilderConfig() MetricsBuilderConfig {
	return MetricsBuilderConfig{
		ResourceAttributes: DefaultResourceAttributesSettings(),
	}
}

// MetricsBuilderConfig is a structural subset of an otherwise 1-1 copy of metadata.yaml
type MetricsBuilderConfig struct {
	ResourceAttributes ResourceAttributesSettings `mapstructure:"resource_attributes"`
}

// MetricsBuilder provides an interface for scrapers to report metrics while taking care of all the transformations
// required to produce metric representation defined in metadata and user settings.
type MetricsBuilder struct {
	startTime                  pcommon.Timestamp   // start time that will be applied to all recorded data points.
	metricsCapacity            int                 // maximum observed number of metrics per resource.
	resourceCapacity           int                 // maximum observed number of resource attributes.
	metricsBuffer              pmetric.Metrics     // accumulates metrics data before emitting.
	buildInfo                  component.BuildInfo // contains version information
	resourceAttributesSettings ResourceAttributesSettings
	metrics                    map[string]*metricAzureAbstract
}

// metricBuilderOption applies changes to default metrics builder.
type metricBuilderOption func(*MetricsBuilder)

// WithStartTime sets startTime on the metrics builder.
func WithStartTime(startTime pcommon.Timestamp) metricBuilderOption {
	return func(mb *MetricsBuilder) {
		mb.startTime = startTime
	}
}

// WithResourceAttributesSettings sets ResourceAttributeSettings on the metrics builder.
func WithResourceAttributesSettings(ras ResourceAttributesSettings) metricBuilderOption {
	return func(mb *MetricsBuilder) {
		mb.resourceAttributesSettings = ras
	}
}

// updateCapacity updates max length of metrics and resource attributes that will be used for the slice capacity.
func (mb *MetricsBuilder) updateCapacity(rm pmetric.ResourceMetrics) {
	if mb.metricsCapacity < rm.ScopeMetrics().At(0).Metrics().Len() {
		mb.metricsCapacity = rm.ScopeMetrics().At(0).Metrics().Len()
	}
	if mb.resourceCapacity < rm.Resource().Attributes().Len() {
		mb.resourceCapacity = rm.Resource().Attributes().Len()
	}
}

// ResourceMetricsOption applies changes to provided resource metrics.
type ResourceMetricsOption func(ResourceAttributesSettings, pmetric.ResourceMetrics)

// WithAzureMonitorSubscriptionID sets provided value as "azuremonitor.subscription_id" attribute for current resource.
func WithAzureMonitorSubscriptionID(val string) ResourceMetricsOption {
	return func(ras ResourceAttributesSettings, rm pmetric.ResourceMetrics) {
		if ras.AzureMonitorSubscriptionID.Enabled {
			rm.Resource().Attributes().PutStr("azuremonitor.subscription_id", val)
		}
	}
}

// WithAzuremonitorTenantID sets provided value as "azuremonitor.tenant_id" attribute for current resource.
func WithAzureMonitorTenantID(val string) ResourceMetricsOption {
	return func(ras ResourceAttributesSettings, rm pmetric.ResourceMetrics) {
		if ras.AzureMonitorTenantID.Enabled {
			rm.Resource().Attributes().PutStr("azuremonitor.tenant_id", val)
		}
	}
}

// WithStartTimeOverride overrides start time for all the resource metrics data points.
// This option should be only used if different start time has to be set on metrics coming from different resources.
func WithStartTimeOverride(start pcommon.Timestamp) ResourceMetricsOption {
	return func(ras ResourceAttributesSettings, rm pmetric.ResourceMetrics) {
		var dps pmetric.NumberDataPointSlice
		metrics := rm.ScopeMetrics().At(0).Metrics()
		for i := 0; i < metrics.Len(); i++ {
			switch metrics.At(i).Type() {
			case pmetric.MetricTypeGauge:
				dps = metrics.At(i).Gauge().DataPoints()
			case pmetric.MetricTypeSum:
				dps = metrics.At(i).Sum().DataPoints()
			}
			for j := 0; j < dps.Len(); j++ {
				dps.At(j).SetStartTimestamp(start)
			}
		}
	}
}

// EmitForResource saves all the generated metrics under a new resource and updates the internal state to be ready for
// recording another set of data points as part of another resource. This function can be helpful when one scraper
// needs to emit metrics from several resources. Otherwise calling this function is not required,
// just `Emit` function can be called instead.
// Resource attributes should be provided as ResourceMetricsOption arguments.
func (mb *MetricsBuilder) EmitForResource(rmo ...ResourceMetricsOption) {
	rm := pmetric.NewResourceMetrics()
	rm.Resource().Attributes().EnsureCapacity(mb.resourceCapacity)
	ils := rm.ScopeMetrics().AppendEmpty()
	ils.Scope().SetName("otelcol/azuremonitorreceiver")
	ils.Scope().SetVersion(mb.buildInfo.Version)
	ils.Metrics().EnsureCapacity(mb.metricsCapacity)
	mb.EmitAllMetrics(ils)

	for _, op := range rmo {
		op(mb.resourceAttributesSettings, rm)
	}
	if ils.Metrics().Len() > 0 {
		mb.updateCapacity(rm)
		rm.MoveTo(mb.metricsBuffer.ResourceMetrics().AppendEmpty())
	}
}

// Emit returns all the metrics accumulated by the metrics builder and updates the internal state to be ready for
// recording another set of metrics. This function will be responsible for applying all the transformations required to
// produce metric representation defined in metadata and user settings, e.g. delta or cumulative.
func (mb *MetricsBuilder) Emit(rmo ...ResourceMetricsOption) pmetric.Metrics {
	mb.EmitForResource(rmo...)
	metrics := mb.metricsBuffer
	mb.metricsBuffer = pmetric.NewMetrics()
	return metrics
}

// Reset resets metrics builder to its initial state. It should be used when external metrics source is restarted,
// and metrics builder should update its startTime and reset it's internal state accordingly.
func (mb *MetricsBuilder) Reset(options ...metricBuilderOption) {
	mb.startTime = pcommon.NewTimestampFromTime(time.Now())
	for _, op := range options {
		op(mb)
	}
}

type metricAzureAbstract struct {
	data     pmetric.Metric // data buffer for generated metric.
	capacity int            // max observed number of data points added to the metric.
}

func (m *metricAzureAbstract) updateCapacity() {
	if m.data.Gauge().DataPoints().Len() > m.capacity {
		m.capacity = m.data.Gauge().DataPoints().Len()
	}
}

func (m *metricAzureAbstract) init(name, unit string) {
	m.data.SetName(name)
	m.data.SetUnit(unit)
	m.data.SetEmptyGauge()
	m.data.Gauge().DataPoints().EnsureCapacity(m.capacity)
}

func (mb *MetricsBuilder) getMetric(resourceMetricId string) (*metricAzureAbstract, bool) {
	if _, exists := mb.metrics[resourceMetricId]; !exists {
		return nil, false
	}
	return mb.metrics[resourceMetricId], true
}

func (mb *MetricsBuilder) addMetric(resourceMetricId, resourceId, logicalMetricId, unit string) (*metricAzureAbstract, error) {
	if _, exists := mb.metrics[resourceMetricId]; exists {
		return nil, errors.New("metric already exists")
	}

	m := &metricAzureAbstract{}
	m.data = pmetric.NewMetric()

	m.init(logicalMetricId, unit)

	mb.metrics[resourceMetricId] = m

	return mb.metrics[resourceMetricId], nil
}

func (mb *MetricsBuilder) AddDataPoint(resourceId, metric, aggregation, unit string, ts pcommon.Timestamp, val float64) {
	logicalMetricId := getLogicalMetricId(metric, aggregation)
	resourceMetricId := getLogicalResourceMetricId(resourceId, logicalMetricId)

	m, exists := mb.getMetric(resourceMetricId)
	if !exists {
		var err error
		m, err = mb.addMetric(resourceMetricId, resourceId, logicalMetricId, unit)
		if err != nil {
			log.Println(err)
		}
	}
	dp := m.data.Gauge().DataPoints().AppendEmpty()
	dp.SetStartTimestamp(mb.startTime)
	dp.SetTimestamp(ts)
	dp.SetDoubleValue(val)
	dp.Attributes().PutStr("resource.id", resourceId)
}

func getLogicalMetricId(metric, aggregation string) string {
	return strings.ToLower(fmt.Sprintf("azure_%s_%s", strings.ReplaceAll(metric, " ", "_"), aggregation))
}

func getLogicalResourceMetricId(resourceId, logicalMetricId string) string {
	return fmt.Sprintf("%s/%s", strings.ToLower(resourceId), logicalMetricId)
}

func (mb *MetricsBuilder) EmitAllMetrics(ils pmetric.ScopeMetrics) {
	for _, m := range mb.metrics {
		if m.data.Gauge().DataPoints().Len() > 0 {
			metrics := ils.Metrics()
			m.updateCapacity()
			name := m.data.Name()
			unit := m.data.Unit()
			m.data.MoveTo(metrics.AppendEmpty())
			m.init(name, unit)
		}
	}
}
