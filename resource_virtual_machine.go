package main

import (
	"fmt"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

func resourceVirtualMachine() *schema.Resource {
	return &schema.Resource{
		Create: resourceVirtualMachineCreate,
		Read:   resourceVirtualMachineRead,
		Delete: resourceVirtualMachineDelete,

		Schema: map[string]*schema.Schema{
			"name": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},
			"source": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},
			"datacenter": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},
			"folder": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},
			"host": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},
			"pool": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},
			"linked_clone": &schema.Schema{
				Type:     schema.TypeBool,
				Optional: true,
			},
			"power_on": &schema.Schema{
				Type:     schema.TypeBool,
				Optional: true,
                Default:  true,
			},
		},
	}
}

func resourceVirtualMachineCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*govmomi.Client)

	ref, err := client.SearchIndex().FindByInventoryPath(fmt.Sprintf("%s/vm/%s", d.Get("datacenter").(string), d.Get("source").(string)))
	if err != nil {
		return fmt.Errorf("Error reading vm: %s", err)
	}
	vm, ok := ref.(*govmomi.VirtualMachine)
	if !ok {
		return fmt.Errorf("Error reading vm")
	}

	ref, err = client.SearchIndex().FindByInventoryPath(fmt.Sprintf("%v/vm/%v", d.Get("datacenter").(string), d.Get("folder").(string)))
	if err != nil {
		return fmt.Errorf("Error reading folder: %s", err)
	}
	f, ok := ref.(*govmomi.Folder)
	if !ok {
		return fmt.Errorf("Error reading folder")
	}

	ref, err = client.SearchIndex().FindByInventoryPath(fmt.Sprintf("%v/host/%v/Resources/%v", d.Get("datacenter").(string), d.Get("host").(string), d.Get("pool").(string)))
	if err != nil {
		return fmt.Errorf("Error reading resource pool: %s", err)
	}
	p, ok := ref.(*govmomi.ResourcePool)
	if !ok {
		return fmt.Errorf("Error reading resource pool")
	}
	pref := p.Reference()

	var o mo.VirtualMachine
	err = client.Properties(vm.Reference(), []string{"snapshot"}, &o)
	if err != nil {
		return fmt.Errorf("Error reading snapshot")
	}
	if o.Snapshot == nil {
		return fmt.Errorf("Base VM has no snapshots")
	}
	sref := o.Snapshot.CurrentSnapshot

	relocateSpec := types.VirtualMachineRelocateSpec{
		Pool: &pref,
	}
	linkedClone := d.Get("linked_clone").(bool)
	if linkedClone {
		relocateSpec.DiskMoveType = "createNewChildDiskBacking"
	}
	cloneSpec := types.VirtualMachineCloneSpec{
		Snapshot: sref,
		Location: relocateSpec,
        PowerOn: d.Get("power_on").(bool),
	}
	name := d.Get("name").(string)

	task, err := vm.Clone(f, name, cloneSpec)
	if err != nil {
		return fmt.Errorf("Error clonning vm: %s", err)
	}
	info, err := task.WaitForResult(nil)
	if err != nil {
		return fmt.Errorf("Error clonning vm: %s", err)
	}

	d.SetId(info.Result.(types.ManagedObjectReference).Value)
	return nil
}

func resourceVirtualMachineRead(d *schema.ResourceData, meta interface{}) error {
    client := meta.(*govmomi.Client)
    ref := types.ManagedObjectReference{Type: "VirtualMachine", Value: d.Id() }
    vm := govmomi.NewVirtualMachine(client, ref)

    var o mo.VirtualMachine
    err := client.Properties(vm.Reference(), []string{"config.name"}, &o)
    if err != nil {
        d.SetId("")
        return nil
    }
    d.Set("name", o.Config.Name)

	return nil
}

func resourceVirtualMachineDelete(d *schema.ResourceData, meta interface{}) error {
    client := meta.(*govmomi.Client)
    ref := types.ManagedObjectReference{Type: "VirtualMachine", Value: d.Id() }
    vm := govmomi.NewVirtualMachine(client, ref)

    task, err := vm.PowerOff()
    if err != nil {
        return fmt.Errorf("Error powering vm off: %s", err)
    }
    task.WaitForResult(nil)

    task, err = vm.Destroy()
    if err != nil {
        return fmt.Errorf("Error deleting vm: %s", err)
    }
    _, err = task.WaitForResult(nil)
    if err != nil {
        return fmt.Errorf("Error deleting vm: %s", err)
    }

    return nil
}