package proxmox6

import (
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/tidwall/gjson"
)

func resourcePool() *schema.Resource {
	return &schema.Resource{
		Create: resourcePoolCreate,
		Read:   resourcePoolRead,
		Update: resourcePoolUpdate,
		Delete: resourcePoolDelete,

		Schema: map[string]*schema.Schema{
			"poolid": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},
			"comment": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
		},
	}
}

func resourcePoolCreate(d *schema.ResourceData, m interface{}) error {
	connInfo := m.(*providerConfiguration)

	pxNewRequest(connInfo).
		SetFormData(map[string]string{
			"poolid":  d.Get("poolid").(string),
			"comment": d.Get("comment").(string),
		}).
		Post("/api2/json/pools")

	d.SetId(d.Get("poolid").(string))

	return resourcePoolRead(d, m)
}

func resourcePoolRead(d *schema.ResourceData, m interface{}) error {
	connInfo := m.(*providerConfiguration)

	resp, _ := pxNewRequest(connInfo).
		SetPathParams(map[string]string{
			"poolid": d.Id(),
		}).
		Get("/api2/json/pools/{poolid}")

	if strings.Contains(resp.String(), "does not exist") {
		d.SetId("")
		return nil
	}

	d.Set("comment", gjson.Get(resp.String(), "data.comment").String())
	d.Set("poolid", d.Id())

	return nil
}

func resourcePoolUpdate(d *schema.ResourceData, m interface{}) error {
	connInfo := m.(*providerConfiguration)

	d.Partial(true)

	if d.HasChange("comment") {
		_, err := pxNewRequest(connInfo).
			SetFormData(map[string]string{
				"comment": d.Get("comment").(string),
			}).
			SetPathParams(map[string]string{
				"poolid": d.Id(),
			}).
			Put("/api2/json/pools/{poolid}")

		if err != nil {
			return err
		}

		d.SetPartial("comment")
	}

	d.Partial(false)

	return resourcePoolRead(d, m)
}

func resourcePoolDelete(d *schema.ResourceData, m interface{}) error {
	connInfo := m.(*providerConfiguration)

	pxNewRequest(connInfo).
		SetPathParams(map[string]string{
			"poolid": d.Id(),
		}).
		Delete("/api2/json/pools/{poolid}")

	d.SetId("")

	return nil
}
