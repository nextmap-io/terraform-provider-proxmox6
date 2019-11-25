package proxmox6

import (
        "github.com/hashicorp/terraform-plugin-sdk/helper/schema"
)

func resourcePool() *schema.Resource {
        return &schema.Resource{
                Create: resourcePoolCreate,
                Read:   resourcePoolRead,
                Update: resourcePoolUpdate,
                Delete: resourcePoolDelete,

                Schema: map[string]*schema.Schema{
                        "name": &schema.Schema{
                                Type:     schema.TypeString,
                                Required: true,
                        },
                },
        }
}

func resourcePoolCreate(d *schema.ResourceData, m interface{}) error {
	return resourcePoolRead(d, m)
}

func resourcePoolRead(d *schema.ResourceData, m interface{}) error {
	return nil
}

func resourcePoolUpdate(d *schema.ResourceData, m interface{}) error {
	return resourcePoolRead(d, m)
}

func resourcePoolDelete(d *schema.ResourceData, m interface{}) error {
	return nil
}