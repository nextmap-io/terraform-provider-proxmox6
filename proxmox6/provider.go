package proxmox6

import (
	"crypto/tls"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"sync"

	"github.com/go-resty/resty/v2"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
)

var log = logrus.New()

type credentials struct {
	CSRFPreventionToken string `json:"CSRFPreventionToken"`
	ticket              string `json:"ticket"`
}

type providerConfiguration struct {
	Client          *resty.Client
	Creds           *credentials
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
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("PM_USER", nil),
				Description: "username, maywith with @pam",
			},
			"pm_password": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("PM_PASS", nil),
				Description: "secret",
				Sensitive:   true,
			},
			"pm_api_url": {
				Type:        schema.TypeString,
				Optional:    true,
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

		ConfigureFunc: providerConfigure,

		ResourcesMap: map[string]*schema.Resource{
			"proxmox6_pool": resourcePool(),
		},
	}
}

func providerConfigure(d *schema.ResourceData) (interface{}, error) {
	file, ferr := os.OpenFile("logrus.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if ferr == nil {
		log.Out = file
	} else {
		log.Info("Failed to log to file, using default stderr")
	}
	log.SetFormatter(&logrus.TextFormatter{
		DisableColors: true,
		FullTimestamp: true,
	})

	log.SetLevel(logrus.DebugLevel)
	log.Info("Initializing provider")
	pmAPIURL := d.Get("pm_api_url").(string)
	pmUser := d.Get("pm_user").(string)
	pmPassword := d.Get("pm_password").(string)
	pmTLSInsecure := d.Get("pm_tls_insecure").(bool)
	pmOtp := d.Get("pm_otp").(string)

	log.Debug("Getting a Proxmox ticket")
	pxCreds := credentials{}

	client := resty.New().
		SetLogger(log).
		SetHostURL(pmAPIURL).
		SetDebug(true)

	if pmTLSInsecure {
		client.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})
	}

	resp, err := client.R().
		SetFormData(map[string]string{"username": pmUser, "password": pmPassword, "otp": pmOtp}).
		Post("/api2/json/access/ticket")

	log.Debugf("%v", resp.String())

	pxCreds.CSRFPreventionToken = gjson.Get(resp.String(), "data.CSRFPreventionToken").String()
	pxCreds.ticket = gjson.Get(resp.String(), "data.ticket").String()

	if err != nil {
		log.Fatal(err)
		return nil, err
	}

	if pxCreds.ticket == "" {
		log.Fatal("No ticket received, error")
		return nil, fmt.Errorf("No ticket received, error")
	}

	var mut sync.Mutex

	return &providerConfiguration{
		Client:          client,
		Creds:           &pxCreds,
		MaxParallel:     d.Get("pm_parallel").(int),
		CurrentParallel: 0,
		MaxVMID:         -1,
		Mutex:           &mut,
		Cond:            sync.NewCond(&mut),
	}, nil

}

func pxNewRequest(providerConfiguration *providerConfiguration) *resty.Request {
	requestClient := providerConfiguration.Client.R().
		SetHeader("Cookie", fmt.Sprintf("PVEAuthCookie=%s", providerConfiguration.Creds.ticket)).
		SetHeader("CSRFPreventionToken", providerConfiguration.Creds.CSRFPreventionToken)

	return requestClient
}

func nextVMID(pconf *providerConfiguration) (nextID int, err error) {
	pconf.Mutex.Lock()
	//pconf.MaxVMID, err = pconf.Client.GetnextID(pconf.MaxVMID + 1)
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
