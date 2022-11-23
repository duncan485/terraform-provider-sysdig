package sysdig

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"

	"github.com/draios/terraform-provider-sysdig/sysdig/internal/client/monitor"
)

func resourceSysdigMonitorAlertV2Metric() *schema.Resource {

	timeout := 5 * time.Minute

	return &schema.Resource{
		CreateContext: resourceSysdigMonitorAlertV2MetricCreate,
		UpdateContext: resourceSysdigMonitorAlertV2MetricUpdate,
		ReadContext:   resourceSysdigMonitorAlertV2MetricRead,
		DeleteContext: resourceSysdigMonitorAlertV2MetricDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(timeout),
			Update: schema.DefaultTimeout(timeout),
			Read:   schema.DefaultTimeout(timeout),
			Delete: schema.DefaultTimeout(timeout),
		},

		Schema: createScopedSegmentedAlertV2Schema(createAlertV2Schema(map[string]*schema.Schema{
			"op": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.StringInSlice([]string{">", ">=", "<", "<=", "=", "!="}, false),
			},
			"threshold": {
				Type:     schema.TypeFloat,
				Required: true,
			},
			"warning_threshold": {
				Type:     schema.TypeFloat,
				Optional: true,
			},
			"metric": {
				Type:     schema.TypeString,
				Required: true,
			},
			"time_aggregation": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.StringInSlice([]string{"avg", "timeAvg", "sum", "min", "max"}, false),
			},
			"group_aggregation": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.StringInSlice([]string{"avg", "sum", "min", "max"}, false),
			},
			"no_data_behaviour": {
				Type:         schema.TypeString,
				Optional:     true,
				Default:      "DO_NOTHING",
				ValidateFunc: validation.StringInSlice([]string{"DO_NOTHING", "TRIGGER"}, false),
			},
		})),
	}
}

func resourceSysdigMonitorAlertV2MetricCreate(ctx context.Context, d *schema.ResourceData, i interface{}) diag.Diagnostics {
	client, err := i.(SysdigClients).sysdigMonitorClient()
	if err != nil {
		return diag.FromErr(err)
	}

	a, err := buildAlertV2MetricStruct(ctx, d, client)
	if err != nil {
		return diag.FromErr(err)
	}

	aCreated, err := client.CreateAlertV2Metric(ctx, *a)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(strconv.Itoa(aCreated.ID))

	err = updateAlertV2MetricState(d, &aCreated)
	if err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func resourceSysdigMonitorAlertV2MetricRead(ctx context.Context, d *schema.ResourceData, i interface{}) diag.Diagnostics {
	client, err := i.(SysdigClients).sysdigMonitorClient()
	if err != nil {
		return diag.FromErr(err)
	}

	id, err := strconv.Atoi(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	a, err := client.GetAlertV2MetricById(ctx, id)

	if err != nil {
		d.SetId("")
		return nil
	}

	err = updateAlertV2MetricState(d, &a)
	if err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func resourceSysdigMonitorAlertV2MetricUpdate(ctx context.Context, d *schema.ResourceData, i interface{}) diag.Diagnostics {
	client, err := i.(SysdigClients).sysdigMonitorClient()
	if err != nil {
		return diag.FromErr(err)
	}

	a, err := buildAlertV2MetricStruct(ctx, d, client)
	if err != nil {
		return diag.FromErr(err)
	}

	a.ID, _ = strconv.Atoi(d.Id())

	aUpdated, err := client.UpdateAlertV2Metric(ctx, *a)
	if err != nil {
		return diag.FromErr(err)
	}

	err = updateAlertV2MetricState(d, &aUpdated)
	if err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func resourceSysdigMonitorAlertV2MetricDelete(ctx context.Context, d *schema.ResourceData, i interface{}) diag.Diagnostics {
	client, err := i.(SysdigClients).sysdigMonitorClient()
	if err != nil {
		return diag.FromErr(err)
	}

	id, err := strconv.Atoi(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	err = client.DeleteAlertV2Metric(ctx, id)
	if err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func buildAlertV2MetricStruct(ctx context.Context, d *schema.ResourceData, client monitor.SysdigMonitorClient) (*monitor.AlertV2Metric, error) {
	alertV2Common, err := buildAlertV2CommonStruct(ctx, d, client)
	if err != nil {
		return nil, err
	}
	alertV2Common.Type = monitor.AlertV2AlertType_Manual
	config := &monitor.AlertV2ConfigMetric{}

	err = buildScopedSegmentedConfigStruct(ctx, d, client, &config.ScopedSegmentedConfig)
	if err != nil {
		return nil, err
	}

	//ConditionOperator
	config.ConditionOperator = d.Get("op").(string)

	//threshold
	config.Threshold = d.Get("threshold").(float64)

	//WarningThreshold
	if warningThreshold, ok := d.GetOk("warning_threshold"); ok {
		wt := warningThreshold.(float64)
		config.WarningThreshold = &wt
		config.WarningConditionOperator = config.ConditionOperator
	}

	//TimeAggregation
	config.TimeAggregation = d.Get("time_aggregation").(string)

	//GroupAggregation
	config.GroupAggregation = d.Get("group_aggregation").(string)

	//Metric
	metric := d.Get("metric").(string)
	labelDescriptorV3, err := client.GetLabelDescriptor(ctx, metric)
	if err != nil {
		return nil, fmt.Errorf("error getting descriptor for label %s: %w", metric, err)
	}
	config.Metric.ID = labelDescriptorV3.LabelDescriptor.ID
	config.Metric.PublicID = labelDescriptorV3.LabelDescriptor.PublicID

	config.NoDataBehaviour = d.Get("no_data_behaviour").(string)

	alert := &monitor.AlertV2Metric{
		AlertV2Common: *alertV2Common,
		Config:        config,
	}
	return alert, nil
}

func updateAlertV2MetricState(d *schema.ResourceData, alert *monitor.AlertV2Metric) error {
	err := updateAlertV2CommonState(d, &alert.AlertV2Common)
	if err != nil {
		return err
	}

	err = updateScopedSegmentedConfigState(d, &alert.Config.ScopedSegmentedConfig)
	if err != nil {
		return err
	}

	_ = d.Set("op", alert.Config.ConditionOperator)

	_ = d.Set("threshold", alert.Config.Threshold)

	if alert.Config.WarningThreshold != nil {
		_ = d.Set("warning_threshold", alert.Config.WarningThreshold)
	}

	_ = d.Set("time_aggregation", alert.Config.TimeAggregation)

	_ = d.Set("group_aggregation", alert.Config.GroupAggregation)

	_ = d.Set("metric", alert.Config.Metric.PublicID)

	_ = d.Set("no_data_behaviour", alert.Config.NoDataBehaviour)

	return nil
}