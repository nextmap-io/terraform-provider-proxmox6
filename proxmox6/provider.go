package proxmox6

import (
		"crypto/tls"
		"fmt"
		"os"
		"regexp"
		"strconv"
		"sync"

		pxapi "github.com/Telmate/proxmox-api-go/proxmox"
		"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
)

type providerConfiguration struct {
	Client          *pxapi.Client
	MaxParallel     int
	CurrentParallel int
	MaxVMID         int
	Mutex           *sync.Mutex
	Cond            *sync.Cond
}

func Provider() *schema.Provider {
		pmOTPprompt := schema.Schema{
			Type:        schema.TypeString,
			Optional:    true,
			DefaultFunc: schema.EnvDefaultFunc("PM_OTP", ""),
			Description: "OTP 2FA code (if required)",
		}
		if os.Getenv("PM_OTP_PROMPT") == "1" {
			pmOTPprompt = schema.Schema{
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("PM_OTP", nil),
				Description: "OTP 2FA code (if required)",
			}
		}

        return &schema.Provider{
				Schema: map[string]*schema.Schema{

					"pm_user": {
						Type:        schema.TypeString,
						Required:    true,
						DefaultFunc: schema.EnvDefaultFunc("PM_USER", nil),
						Description: "username, maywith with @pam",
					},
					"pm_password": {
						Type:        schema.TypeString,
						Required:    true,
						DefaultFunc: schema.EnvDefaultFunc("PM_PASS", nil),
						Description: "secret",
						Sensitive:   true,
					},
					"pm_api_url": {
						Type:        schema.TypeString,
						Required:    true,
						DefaultFunc: schema.EnvDefaultFunc("PM_API_URL", nil),
						Description: "https://host.fqdn:8006/api2/json",
					},
					"pm_parallel": {
						Type:     schema.TypeInt,
						Optional: true,
						Default:  4,
					},
					"pm_tls_insecure": {
						Type:     schema.TypeBool,
						Optional: true,
						Default:  false,
					},
					"pm_otp": &pmOTPprompt,
				},

				ResourcesMap: map[string]*schema.Resource{
					"proxmox6_pool": resourcePool(),
				},
        }
}

func providerConfigure(d *schema.ResourceData) (interface{}, error) {
	client, err := getClient(d.Get("pm_api_url").(string), d.Get("pm_user").(string), d.Get("pm_password").(string), d.Get("pm_otp").(string), d.Get("pm_tls_insecure").(bool))
	if err != nil {
		return nil, err
	}
	var mut sync.Mutex
	return &providerConfiguration{
		Client:          client,
		MaxParallel:     d.Get("pm_parallel").(int),
		CurrentParallel: 0,
		MaxVMID:         -1,
		Mutex:           &mut,
		Cond:            sync.NewCond(&mut),
	}, nil
}

func getClient(pm_api_url string, pm_user string, pm_password string, pm_otp string, pm_tls_insecure bool) (*pxapi.Client, error) {
	tlsconf := &tls.Config{InsecureSkipVerify: true}
	if !pm_tls_insecure {
		tlsconf = nil
	}
	client, _ := pxapi.NewClient(pm_api_url, nil, tlsconf)
	err := client.Login(pm_user, pm_password, pm_otp)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func nextVMID(pconf *providerConfiguration) (nextID int, err error) {
	pconf.Mutex.Lock()
	pconf.MaxVMID, err = pconf.Client.GetnextID(pconf.MaxVMID + 1)
	if err != nil {
		return 0, err
	}
	nextID = pconf.MaxVMID
	pconf.Mutex.Unlock()
	return nextID, nil
}

func pmParallelBegin(pconf *providerConfiguration) {
	pconf.Mutex.Lock()
	for pconf.CurrentParallel >= pconf.MaxParallel {
		pconf.Cond.Wait()
	}
	pconf.CurrentParallel++
	pconf.Mutex.Unlock()
}

func pmParallelEnd(pconf *providerConfiguration) {
	pconf.Mutex.Lock()
	pconf.CurrentParallel--
	pconf.Cond.Signal()
	pconf.Mutex.Unlock()
}

func resourceID(targetNode string, resType string, vmID int) string {
	return fmt.Sprintf("%s/%s/%d", targetNode, resType, vmID)
}

var rxRsID = regexp.MustCompile("([^/]+)/([^/]+)/(\\d+)")

func parseResourceID(resID string) (targetNode string, resType string, vmID int, err error) {
	if !rxRsID.MatchString(resID) {
		return "", "", -1, fmt.Errorf("Invalid resource format: %s. Must be node/type/vmID", resID)
	}
	idMatch := rxRsID.FindStringSubmatch(resID)
	targetNode = idMatch[1]
	resType = idMatch[2]
	vmID, err = strconv.Atoi(idMatch[3])
	return
}