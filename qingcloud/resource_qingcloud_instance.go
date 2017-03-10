package qingcloud

import (
	"fmt"

	"github.com/hashicorp/terraform/helper/schema"

	qc "github.com/yunify/qingcloud-sdk-go/service"
)

func resourceQingcloudInstance() *schema.Resource {
	return &schema.Resource{
		Create: resourceQingcloudInstanceCreate,
		Read:   resourceQingcloudInstanceRead,
		Update: resourceQingcloudInstanceUpdate,
		Delete: resourceQingcloudInstanceDelete,
		Schema: map[string]*schema.Schema{
			"name": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},
			"description": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
			"image_id": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"instance_type": &schema.Schema{
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: withinArrayString("c1m1", "c1m2", "c1m4", "c2m2", "c2m4", "c2m8", "c4m4", "c4m8"),
			},
			"instance_class": &schema.Schema{
				Type:         schema.TypeInt,
				Default:      0,
				Optional:     true,
				ForceNew:     true,
				ValidateFunc: withinArrayInt(0, 1),
			},
			"instance_state": &schema.Schema{
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: withinArrayString("pending", "running", "stopped", "suspended", "terminated", "ceased"),
			},
			"cpu": &schema.Schema{
				Type:     schema.TypeInt,
				Optional: true,
			},
			"memory": &schema.Schema{
				Type:     schema.TypeInt,
				Optional: true,
			},
			"vxnet_id": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},
			"hostname": &schema.Schema{
				Type:     schema.TypeString,
				ForceNew: true,
				Optional: true,
			},
			"keypair_ids": &schema.Schema{
				Type:     schema.TypeSet,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Set:      schema.HashString,
			},
			"security_group_id": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},
			"eip_id": &schema.Schema{
				Type:     schema.TypeString,
				Computed: true,
			},
			"public_ip": &schema.Schema{
				Type:     schema.TypeString,
				Computed: true,
			},
			"private_ip": &schema.Schema{
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

func resourceQingcloudInstanceCreate(d *schema.ResourceData, meta interface{}) error {
	clt := meta.(*QingCloudClient).instance
	input := new(qc.RunInstancesInput)
	input.ImageID = qc.String(d.Get("image_id").(string))
	input.InstanceClass = qc.Int(d.Get("instance_class").(int))
	input.InstanceType = qc.String(d.Get("instance_type").(string))
	input.CPU = qc.Int(d.Get("cpu").(int))
	input.Memory = qc.Int(d.Get("memory").(int))
	input.VxNets = []*string{qc.String(d.Get("vxnet_id").(string))}
	input.SecurityGroup = qc.String(d.Get("security_group").(string))
	input.LoginMode = qc.String("keypair")
	kps := d.Get("keypair_ids").(*schema.Set).List()
	if len(kps) > 0 {
		kp := kps[0].(string)
		input.LoginKeyPair = qc.String(kp)
	}
	err := input.Validate()
	if err != nil {
		return fmt.Errorf("Error run intances input validate: %s", err)
	}
	output, err := clt.RunInstances(input)
	if err != nil {
		return fmt.Errorf("Error run instances: %s", err)
	}
	if output.RetCode != nil && qc.IntValue(output.RetCode) != 0 {
		return fmt.Errorf("Error run instances: %s", *output.Message)
	}
	d.SetId(qc.StringValue(output.Instances[0]))
	if _, err := InstanceTransitionStateRefresh(clt, d.Id()); err != nil {
		return err
	}
	err = modifyInstanceAttributes(d, meta, true)
	if err != nil {
		return err
	}
	// associate eip to instance
	if eipID := d.Get("eip_id").(string); eipID != "" {
		eipClt := meta.(*QingCloudClient).eip
		if _, err := EIPTransitionStateRefresh(eipClt, eipID); err != nil {
			return err
		}
		associateEIPInput := new(qc.AssociateEIPInput)
		associateEIPInput.EIP = qc.String(eipID)
		associateEIPInput.Instance = qc.String(d.Id())
		err := associateEIPInput.Validate()
		if err != nil {
			return fmt.Errorf("Error associate eip input validate: %s", err)
		}
		associateEIPoutput, err := eipClt.AssociateEIP(associateEIPInput)
		if err != nil {
			return fmt.Errorf("Error associate eip: %s", err)
		}
		if associateEIPoutput.RetCode != nil && qc.IntValue(associateEIPoutput.RetCode) != 0 {
			return fmt.Errorf("Error associate eip: %s", *associateEIPoutput.Message)
		}
		if _, err := EIPTransitionStateRefresh(eipClt, eipID); err != nil {
			return err
		}
	}

	return resourceQingcloudInstanceRead(d, meta)
}

func resourceQingcloudInstanceRead(d *schema.ResourceData, meta interface{}) error {
	clt := meta.(*QingCloudClient).instance
	input := new(qc.DescribeInstancesInput)
	input.Instances = []*string{qc.String(d.Id())}
	input.Verbose = qc.Int(1)
	err := input.Validate()
	if err != nil {
		return fmt.Errorf("Error describe instance input validate: %s", err)
	}
	output, err := clt.DescribeInstances(input)
	if err != nil {
		return fmt.Errorf("Error describe instance: %s", err)
	}
	if output.RetCode != nil && qc.IntValue(output.RetCode) != 0 {
		return fmt.Errorf("Error describe instance: %s", *output.Message)
	}

	instance := output.InstanceSet[0]
	d.Set("name", qc.StringValue(instance.InstanceName))
	d.Set("description", qc.StringValue(instance.Description))
	d.Set("image_id", qc.StringValue(instance.ImageID))
	d.Set("instance_type", qc.StringValue(instance.InstanceType))
	d.Set("instance_class", qc.IntValue(instance.InstanceClass))
	d.Set("instance_state", qc.StringValue(instance.Status))
	d.Set("cpu", qc.IntValue(instance.VCPUsCurrent))
	d.Set("memory", qc.IntValue(instance.MemoryCurrent))
	if instance.VxNets != nil && len(instance.VxNets) > 0 {
		vxnet := instance.VxNets[0]
		d.Set("vxnet_id", qc.StringValue(vxnet.VxNetID))
		d.Set("private_ip", qc.StringValue(vxnet.PrivateIP))
	}
	if instance.EIP != nil {
		d.Set("eip_id", qc.StringValue(instance.EIP.EIPID))
		d.Set("public_ip", qc.StringValue(instance.EIP.EIPAddr))
	}
	if instance.SecurityGroup != nil {
		d.Set("security_group_id", qc.StringValue(instance.SecurityGroup.SecurityGroupID))
	}
	if instance.KeyPairIDs != nil {
		keypairIDs := make([]string, 0, len(instance.KeyPairIDs))
		for _, kp := range instance.KeyPairIDs {
			keypairIDs = append(keypairIDs, qc.StringValue(kp))
		}
		d.Set("keypair_ids", keypairIDs)
	}
	return nil
}

func resourceQingcloudInstanceUpdate(d *schema.ResourceData, meta interface{}) error {
	// clt := meta.(*QingCloudClient).instance
	err := modifyInstanceAttributes(d, meta, false)
	if err != nil {
		return err
	}
	// change vxnet

	// change security_group

	// change eip

	// change keypair

	// resize instance

	return resourceQingcloudInstanceRead(d, meta)
}

func resourceQingcloudInstanceDelete(d *schema.ResourceData, meta interface{}) error {
	clt := meta.(*QingCloudClient).instance
	if _, err := InstanceTransitionStateRefresh(clt, d.Id()); err != nil {
		return err
	}
	input := new(qc.TerminateInstancesInput)
	input.Instances = []*string{qc.String(d.Id())}
	err := input.Validate()
	if err != nil {
		return fmt.Errorf("Error terminate instance input validate: %s", err)
	}
	output, err := clt.TerminateInstances(input)
	if err != nil {
		return fmt.Errorf("Error terminate instance: %s", err)
	}
	if output.RetCode != nil && qc.IntValue(output.RetCode) != 0 {
		return fmt.Errorf("Error terminate instance: %s", *output.Message)
	}
	d.SetId("")
	return nil
}
