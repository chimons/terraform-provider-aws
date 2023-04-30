package vpclattice

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/vpclattice"
	"github.com/aws/aws-sdk-go-v2/service/vpclattice/types"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/hashicorp/terraform-provider-aws/internal/conns"
	"github.com/hashicorp/terraform-provider-aws/internal/create"
	"github.com/hashicorp/terraform-provider-aws/internal/enum"
	"github.com/hashicorp/terraform-provider-aws/internal/tfresource"
	"github.com/hashicorp/terraform-provider-aws/names"
)

// Function annotations are used for resource registration to the Provider. DO NOT EDIT.
// @SDKResource("aws_vpclattice_register_targets", name="Register Targets")
func ResourceRegisterTargets() *schema.Resource {
	return &schema.Resource{
		CreateWithoutTimeout: resourceRegisterTargetsCreate,
		ReadWithoutTimeout:   resourceRegisterTargetsRead,
		DeleteWithoutTimeout: resourceRegisterTargetsDelete,

		Importer: &schema.ResourceImporter{
			StateContext: func(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
				idParts := strings.Split(d.Id(), "/")
				d.Set("target_group_identifier", idParts[0])

				return []*schema.ResourceData{d}, nil
			},
		},

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(30 * time.Minute),
			Delete: schema.DefaultTimeout(30 * time.Minute),
		},

		Schema: map[string]*schema.Schema{
			"target_group_identifier": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"targets": {
				Type:     schema.TypeList,
				Optional: true,
				ForceNew: true,
				MaxItems: 1,
				MinItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"id": {
							Type:         schema.TypeString,
							Optional:     true,
							ForceNew:     true,
							ValidateFunc: validation.StringLenBetween(1, 2048),
						},
						"port": {
							Type:     schema.TypeInt,
							Optional: true,
							ForceNew: true,
						},
					},
				},
			},
		},
	}
}

const (
	ResNameRegisterTargets = "Register Targets"
)

func resourceRegisterTargetsCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	conn := meta.(*conns.AWSClient).VPCLatticeClient()

	in := &vpclattice.RegisterTargetsInput{
		TargetGroupIdentifier: aws.String(d.Get("target_group_identifier").(string)),
	}

	var targetId string
	if v, ok := d.GetOk("targets"); ok && len(v.([]interface{})) > 0 && v.([]interface{}) != nil {
		targets := expandTargets(v.([]interface{}))

		if len(targets) > 0 {
			target := targets[0]
			log.Printf("[INFO] Registering Target %s with Target Group %s", aws.ToString(target.Id), d.Get("target_group_identifier").(string))
			targetId = *target.Id
			in.Targets = targets
		}
	}

	out, err := conn.RegisterTargets(ctx, in)
	if err != nil {
		return create.DiagError(names.VPCLattice, create.ErrActionCreating, ResNameRegisterTargets, d.Get("target_group_identifier").(string), err)
	}

	if out == nil {
		return create.DiagError(names.VPCLattice, create.ErrActionCreating, ResNameRegisterTargets, d.Get("target_group_identifier").(string), errors.New("empty output"))
	}

	targetGroupIdentifier := d.Get("target_group_identifier").(string)
	targets := d.Get("targets").([]interface{})

	parts := []string{
		d.Get("target_group_identifier").(string),
		targetId,
	}

	d.SetId(strings.Join(parts, "/"))

	if _, err := waitRegisterTargets(ctx, conn, targetGroupIdentifier, targets, d.Timeout(schema.TimeoutCreate)); err != nil {
		return create.DiagError(names.VPCLattice, create.ErrActionWaitingForCreation, ResNameRegisterTargets, d.Id(), err)
	}

	return resourceRegisterTargetsRead(ctx, d, meta)
}

func resourceRegisterTargetsRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	conn := meta.(*conns.AWSClient).VPCLatticeClient()

	targetGroupId := d.Get("target_group_identifier").(string)
	targets := d.Get("targets").([]interface{})

	out, err := findRegisterTargets(ctx, conn, targetGroupId, targets)

	if !d.IsNewResource() && tfresource.NotFound(err) {
		log.Printf("[WARN] VpcLattice RegisterTargets (%s) not found, removing from state", d.Id())
		d.SetId("")
		return nil
	}

	if err != nil {
		return create.DiagError(names.VPCLattice, create.ErrActionReading, ResNameRegisterTargets, d.Id(), err)
	}
	d.Set("target_group_identifier", targetGroupId)
	if err := d.Set("targets", flattenTargets(out.Items)); err != nil {
		return diag.FromErr(fmt.Errorf("error setting targets: %s", err))
	}

	return nil
}

func resourceRegisterTargetsDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {

	conn := meta.(*conns.AWSClient).VPCLatticeClient()

	targetGroupIdentifier := d.Get("target_group_identifier").(string)
	targetsRaw := d.Get("targets").([]interface{})
	targets := expandTargets(targetsRaw)

	_, err := conn.DeregisterTargets(ctx, &vpclattice.DeregisterTargetsInput{
		TargetGroupIdentifier: aws.String(targetGroupIdentifier),
		Targets:               targets,
	})

	if err != nil {
		var nfe *types.ResourceNotFoundException
		if errors.As(err, &nfe) {
			return nil
		}

		return create.DiagError(names.VPCLattice, create.ErrActionDeleting, ResNameRegisterTargets, d.Id(), err)
	}
	if _, err := waitDeleteTargets(ctx, conn, targetGroupIdentifier, targetsRaw, d.Timeout(schema.TimeoutDelete)); err != nil {
		return create.DiagError(names.VPCLattice, create.ErrActionWaitingForDeletion, ResNameRegisterTargets, d.Id(), err)
	}

	return nil
}

func findRegisterTargets(ctx context.Context, conn *vpclattice.Client, targetGroupId string, targets []interface{}) (*vpclattice.ListTargetsOutput, error) {
	in := &vpclattice.ListTargetsInput{
		TargetGroupIdentifier: aws.String(targetGroupId),
		Targets:               expandTargets(targets),
	}
	out, err := conn.ListTargets(ctx, in)
	if err != nil {
		var nfe *types.ResourceNotFoundException
		if errors.As(err, &nfe) {
			return nil, &retry.NotFoundError{
				LastError:   err,
				LastRequest: in,
			}
		}

		return nil, err
	}

	if out == nil || out.Items == nil {
		return nil, tfresource.NewEmptyResultError(in)
	}

	return out, nil
}

func waitRegisterTargets(ctx context.Context, conn *vpclattice.Client, id string, targets []interface{}, timeout time.Duration) (*vpclattice.RegisterTargetsOutput, error) {
	stateConf := &retry.StateChangeConf{
		Pending:                   enum.Slice(types.TargetStatusInitial),
		Target:                    enum.Slice(types.TargetStatusHealthy, types.TargetStatusUnhealthy, types.TargetStatusUnused, types.TargetStatusUnavailable),
		Refresh:                   statusTarget(ctx, conn, id, targets),
		Timeout:                   timeout,
		NotFoundChecks:            20,
		ContinuousTargetOccurence: 2,
	}

	outputRaw, err := stateConf.WaitForStateContext(ctx)
	if out, ok := outputRaw.(*vpclattice.RegisterTargetsOutput); ok {
		return out, err
	}

	return nil, err
}

func waitDeleteTargets(ctx context.Context, conn *vpclattice.Client, id string, targets []interface{}, timeout time.Duration) (*vpclattice.DeregisterTargetsOutput, error) {
	stateConf := &retry.StateChangeConf{
		Pending: enum.Slice(types.TargetStatusDraining, types.TargetStatusInitial),
		Target:  []string{},
		Refresh: statusTarget(ctx, conn, id, targets),
		Timeout: timeout,
	}

	outputRaw, err := stateConf.WaitForStateContext(ctx)
	if out, ok := outputRaw.(*vpclattice.DeregisterTargetsOutput); ok {
		return out, err
	}

	return nil, err
}

func statusTarget(ctx context.Context, conn *vpclattice.Client, id string, targets []interface{}) retry.StateRefreshFunc {
	return func() (interface{}, string, error) {
		out, err := findRegisterTargets(ctx, conn, id, targets)
		if tfresource.NotFound(err) {
			return nil, "", nil
		}

		if err != nil {
			return nil, "", err
		}

		var status types.TargetStatus
		if len(out.Items) > 0 {
			status = out.Items[0].Status
			return out, string(status), nil
		}

		return nil, "", err
	}
}

// Flatten function for targets
func flattenTargets(apiObjects []types.TargetSummary) []interface{} {
	if len(apiObjects) == 0 {
		return nil
	}

	var tfList []interface{}

	for _, apiObject := range apiObjects {
		tfList = append(tfList, flattenTarget(&apiObject))
	}

	return tfList
}

func flattenTarget(apiObject *types.TargetSummary) map[string]interface{} {
	if apiObject == nil {
		return nil
	}

	tfMap := map[string]interface{}{}

	if v := apiObject.Id; v != nil {
		tfMap["id"] = aws.ToString(v)
	}

	if v := apiObject.Port; v != nil {
		tfMap["port"] = aws.ToInt32(v)
	}

	return tfMap
}

// Expand function for targets
func expandTargets(tfList []interface{}) []types.Target {
	if len(tfList) == 0 {
		return nil
	}

	var apiObjects []types.Target

	for _, tfMapRaw := range tfList {
		tfMap, ok := tfMapRaw.(map[string]interface{})

		if !ok {
			continue
		}

		apiObject := expandTarget(tfMap)

		apiObjects = append(apiObjects, apiObject)
	}

	return apiObjects
}

func expandTarget(tfMap map[string]interface{}) types.Target {
	apiObject := types.Target{}

	if v, ok := tfMap["id"].(string); ok && v != "" {
		apiObject.Id = aws.String(v)
	}

	if v, ok := tfMap["port"].(int); ok && v != 0 {
		apiObject.Port = aws.Int32(int32(v))
	}

	return apiObject
}
