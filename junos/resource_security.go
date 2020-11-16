package junos

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

type securityOptions struct {
	ikeTraceoptions []map[string]interface{}
	utm             []map[string]interface{}
	alg             []map[string]interface{}
}

func resourceSecurity() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceSecurityCreate,
		ReadContext:   resourceSecurityRead,
		UpdateContext: resourceSecurityUpdate,
		DeleteContext: resourceSecurityDelete,
		Importer: &schema.ResourceImporter{
			State: resourceSecurityImport,
		},
		Schema: map[string]*schema.Schema{
			"ike_traceoptions": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"file": {
							Type:     schema.TypeList,
							Optional: true,
							MaxItems: 1,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"name": {
										Type:     schema.TypeString,
										Optional: true,
									},
									"files": {
										Type:         schema.TypeInt,
										Optional:     true,
										ValidateFunc: validation.IntBetween(2, 1000),
									},
									"match": {
										Type:     schema.TypeString,
										Optional: true,
									},
									"size": {
										Type:         schema.TypeInt,
										Optional:     true,
										ValidateFunc: validation.IntBetween(10240, 1073741824),
									},
									"no_world_readable": {
										Type:     schema.TypeBool,
										Optional: true,
									},
									"world_readable": {
										Type:     schema.TypeBool,
										Optional: true,
									},
								},
							},
						},
						"flag": {
							Type:     schema.TypeList,
							Optional: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
						},
						"no_remote_trace": {
							Type:     schema.TypeBool,
							Optional: true,
						},
						"rate_limit": {
							Type:         schema.TypeInt,
							Optional:     true,
							Default:      -1,
							ValidateFunc: validation.IntBetween(0, 4294967295),
						},
					},
				},
			},
			"utm": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"feature_profile_web_filtering_type": {
							Type:     schema.TypeString,
							Optional: true,
							ValidateFunc: validation.StringInSlice([]string{
								"juniper-enhanced", "juniper-local", "web-filtering-none", "websense-redirect",
							}, false),
						},
					},
				},
			},
			"alg": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"dns_disable": {
							Type:     schema.TypeBool,
							Optional: true,
						},
						"ftp_disable": {
							Type:     schema.TypeBool,
							Optional: true,
						},
						"msrpc_disable": {
							Type:     schema.TypeBool,
							Optional: true,
						},
						"pptp_disable": {
							Type:     schema.TypeBool,
							Optional: true,
						},
						"sunrpc_disable": {
							Type:     schema.TypeBool,
							Optional: true,
						},
						"talk_disable": {
							Type:     schema.TypeBool,
							Optional: true,
						},
						"tftp_disable": {
							Type:     schema.TypeBool,
							Optional: true,
						},
					},
				},
			},
		},
	}
}

func resourceSecurityCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	sess := m.(*Session)
	jnprSess, err := sess.startNewSession()
	if err != nil {
		return diag.FromErr(err)
	}
	defer sess.closeSession(jnprSess)
	if !checkCompatibilitySecurity(jnprSess) {
		return diag.FromErr(fmt.Errorf("security not compatible with Junos device %s", jnprSess.Platform[0].Model))
	}
	sess.configLock(jnprSess)

	if err := setSecurity(d, m, jnprSess); err != nil {
		sess.configClear(jnprSess)

		return diag.FromErr(err)
	}
	if err := sess.commitConf("create resource junos_security", jnprSess); err != nil {
		sess.configClear(jnprSess)

		return diag.FromErr(err)
	}

	d.SetId("security")

	return resourceSecurityRead(ctx, d, m)
}
func resourceSecurityRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	sess := m.(*Session)
	mutex.Lock()
	jnprSess, err := sess.startNewSession()
	if err != nil {
		mutex.Unlock()

		return diag.FromErr(err)
	}
	defer sess.closeSession(jnprSess)
	securityOptions, err := readSecurity(m, jnprSess)
	mutex.Unlock()
	if err != nil {
		return diag.FromErr(err)
	}
	fillSecurity(d, securityOptions)

	return nil
}
func resourceSecurityUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	d.Partial(true)
	sess := m.(*Session)
	jnprSess, err := sess.startNewSession()
	if err != nil {
		return diag.FromErr(err)
	}
	defer sess.closeSession(jnprSess)
	sess.configLock(jnprSess)
	if err := delSecurity(m, jnprSess); err != nil {
		sess.configClear(jnprSess)

		return diag.FromErr(err)
	}
	if err := setSecurity(d, m, jnprSess); err != nil {
		sess.configClear(jnprSess)

		return diag.FromErr(err)
	}
	if err := sess.commitConf("update resource junos_security", jnprSess); err != nil {
		sess.configClear(jnprSess)

		return diag.FromErr(err)
	}
	d.Partial(false)

	return resourceSecurityRead(ctx, d, m)
}
func resourceSecurityDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	return nil
}
func resourceSecurityImport(d *schema.ResourceData, m interface{}) ([]*schema.ResourceData, error) {
	sess := m.(*Session)
	jnprSess, err := sess.startNewSession()
	if err != nil {
		return nil, err
	}
	defer sess.closeSession(jnprSess)
	result := make([]*schema.ResourceData, 1)
	securityOptions, err := readSecurity(m, jnprSess)
	if err != nil {
		return nil, err
	}
	fillSecurity(d, securityOptions)
	d.SetId("security")
	result[0] = d

	return result, nil
}

func setSecurity(d *schema.ResourceData, m interface{}, jnprSess *NetconfObject) error {
	sess := m.(*Session)

	setPrefix := "set security "
	configSet := make([]string, 0)

	for _, ikeTrace := range d.Get("ike_traceoptions").([]interface{}) {
		configSetIkeTrace, err := setSecurityIkeTraceOpts(ikeTrace)
		if err != nil {
			return err
		}
		configSet = append(configSet, configSetIkeTrace...)
	}
	for _, utm := range d.Get("utm").([]interface{}) {
		if utm != nil {
			utmM := utm.(map[string]interface{})
			if utmM["feature_profile_web_filtering_type"].(string) != "" {
				configSet = append(configSet, setPrefix+"utm feature-profile web-filtering type "+
					utmM["feature_profile_web_filtering_type"].(string))
			}
		}
	}
	for _, alg := range d.Get("alg").([]interface{}) {
		configSet = append(configSet, setSecurityAlg(alg)...)
	}
	if err := sess.configSet(configSet, jnprSess); err != nil {
		return err
	}

	return nil
}

func setSecurityIkeTraceOpts(ikeTrace interface{}) ([]string, error) {
	setPrefix := "set security ike traceoptions "
	configSet := make([]string, 0)
	if ikeTrace != nil {
		ikeTraceM := ikeTrace.(map[string]interface{})
		for _, ikeTraceFile := range ikeTraceM["file"].([]interface{}) {
			if ikeTraceFile != nil {
				ikeTraceFileM := ikeTraceFile.(map[string]interface{})
				if ikeTraceFileM["name"].(string) != "" {
					configSet = append(configSet, setPrefix+"file "+
						ikeTraceFileM["name"].(string))
				}
				if ikeTraceFileM["files"].(int) > 0 {
					configSet = append(configSet, setPrefix+"file files "+
						strconv.Itoa(ikeTraceFileM["files"].(int)))
				}
				if ikeTraceFileM["match"].(string) != "" {
					configSet = append(configSet, setPrefix+"file match \""+
						ikeTraceFileM["match"].(string)+"\"")
				}
				if ikeTraceFileM["size"].(int) > 0 {
					configSet = append(configSet, setPrefix+"file size "+
						strconv.Itoa(ikeTraceFileM["size"].(int)))
				}
				if ikeTraceFileM["world_readable"].(bool) && ikeTraceFileM["no_world_readable"].(bool) {
					return configSet, fmt.Errorf("conflict between 'world_readable' and 'no_world_readable' for ike_traceoptions file")
				}
				if ikeTraceFileM["world_readable"].(bool) {
					configSet = append(configSet, setPrefix+"file world-readable")
				}
				if ikeTraceFileM["no_world_readable"].(bool) {
					configSet = append(configSet, setPrefix+"file no-world-readable")
				}
			}
		}
		for _, ikeTraceFlag := range ikeTraceM["flag"].([]interface{}) {
			configSet = append(configSet, setPrefix+"flag "+ikeTraceFlag.(string))
		}
		if ikeTraceM["no_remote_trace"].(bool) {
			configSet = append(configSet, setPrefix+"no-remote-trace")
		}
		if ikeTraceM["rate_limit"].(int) > -1 {
			configSet = append(configSet, setPrefix+"rate-limit "+
				strconv.Itoa(ikeTraceM["rate_limit"].(int)))
		}
	}

	return configSet, nil
}

func setSecurityAlg(alg interface{}) []string {
	setPrefix := "set security alg "
	configSet := make([]string, 0)
	if alg != nil {
		algM := alg.(map[string]interface{})
		if algM["dns_disable"].(bool) {
			configSet = append(configSet, setPrefix+"dns disable")
		}
		if algM["ftp_disable"].(bool) {
			configSet = append(configSet, setPrefix+"ftp disable")
		}
		if algM["msrpc_disable"].(bool) {
			configSet = append(configSet, setPrefix+"msrpc disable")
		}
		if algM["pptp_disable"].(bool) {
			configSet = append(configSet, setPrefix+"pptp disable")
		}
		if algM["sunrpc_disable"].(bool) {
			configSet = append(configSet, setPrefix+"sunrpc disable")
		}
		if algM["talk_disable"].(bool) {
			configSet = append(configSet, setPrefix+"talk disable")
		}
		if algM["tftp_disable"].(bool) {
			configSet = append(configSet, setPrefix+"tftp disable")
		}
	}

	return configSet
}

func listLinesSecurityUtm() []string {
	return []string{
		"utm feature-profile web-filtering type",
	}
}

func listLinesSecurityAlg() []string {
	return []string{
		"alg dns disable",
		"alg ftp disable",
		"alg msrpc disable",
		"alg pptp disable",
		"alg sunrpc disable",
		"alg talk disable",
		"alg tftp disable",
	}
}

func delSecurity(m interface{}, jnprSess *NetconfObject) error {
	listLinesToDelete := []string{
		"ike traceoptions",
	}
	listLinesToDelete = append(listLinesToDelete, listLinesSecurityUtm()...)
	listLinesToDelete = append(listLinesToDelete, listLinesSecurityAlg()...)
	sess := m.(*Session)
	configSet := make([]string, 0)
	delPrefix := "delete security "
	for _, line := range listLinesToDelete {
		configSet = append(configSet,
			delPrefix+line)
	}
	if err := sess.configSet(configSet, jnprSess); err != nil {
		return err
	}

	return nil
}
func readSecurity(m interface{}, jnprSess *NetconfObject) (securityOptions, error) {
	sess := m.(*Session)
	var confRead securityOptions

	securityConfig, err := sess.command("show configuration security"+
		" | display set relative", jnprSess)
	if err != nil {
		return confRead, err
	}
	if securityConfig != emptyWord {
		for _, item := range strings.Split(securityConfig, "\n") {
			if strings.Contains(item, "<configuration-output>") {
				continue
			}
			if strings.Contains(item, "</configuration-output>") {
				break
			}
			itemTrim := strings.TrimPrefix(item, setLineStart)
			switch {
			case strings.HasPrefix(itemTrim, "ike traceoptions"):
				err := readSecurityIkeTraceOptions(&confRead, itemTrim)
				if err != nil {
					return confRead, err
				}
			case checkStringHasPrefixInList(itemTrim, listLinesSecurityUtm()):
				if len(confRead.utm) == 0 {
					confRead.utm = append(confRead.utm, map[string]interface{}{
						"feature_profile_web_filtering_type": "",
					})
				}
				if strings.HasPrefix(itemTrim, "utm feature-profile web-filtering type ") {
					confRead.utm[0]["feature_profile_web_filtering_type"] = strings.TrimPrefix(itemTrim,
						"utm feature-profile web-filtering type ")
				}
			case checkStringHasPrefixInList(itemTrim, listLinesSecurityAlg()):
				readSecurityAlg(&confRead, itemTrim)
			}
		}
	}

	return confRead, nil
}

func readSecurityIkeTraceOptions(confRead *securityOptions, itemTrimIkeTraceOpts string) error {
	itemTrim := strings.TrimPrefix(itemTrimIkeTraceOpts, "ike traceoptions ")
	if len(confRead.ikeTraceoptions) == 0 {
		confRead.ikeTraceoptions = append(confRead.ikeTraceoptions, map[string]interface{}{
			"file":            make([]map[string]interface{}, 0),
			"flag":            make([]string, 0),
			"no_remote_trace": false,
			"rate_limit":      -1,
		})
	}
	switch {
	case strings.HasPrefix(itemTrim, "file"):
		if len(confRead.ikeTraceoptions[0]["file"].([]map[string]interface{})) == 0 {
			confRead.ikeTraceoptions[0]["file"] = append(
				confRead.ikeTraceoptions[0]["file"].([]map[string]interface{}), map[string]interface{}{
					"name":              "",
					"files":             0,
					"match":             "",
					"size":              0,
					"world_readable":    false,
					"no_world_readable": false,
				})
		}
		switch {
		case strings.HasPrefix(itemTrim, "file files"):
			var err error
			confRead.ikeTraceoptions[0]["file"].([]map[string]interface{})[0]["files"], err = strconv.Atoi(
				strings.TrimPrefix(itemTrim, "file files "))
			if err != nil {
				return fmt.Errorf("failed to convert value from '%s' to integer : %w", itemTrim, err)
			}
		case strings.HasPrefix(itemTrim, "file match"):
			confRead.ikeTraceoptions[0]["file"].([]map[string]interface{})[0]["match"] = strings.Trim(
				strings.TrimPrefix(itemTrim, "file match "), "\"")
		case strings.HasPrefix(itemTrim, "file size"):
			var err error
			confRead.ikeTraceoptions[0]["file"].([]map[string]interface{})[0]["size"], err = strconv.Atoi(
				strings.TrimPrefix(itemTrim, "file size "))
			if err != nil {
				return fmt.Errorf("failed to convert value from '%s' to integer : %w", itemTrim, err)
			}
		case strings.HasPrefix(itemTrim, "file world-readable"):
			confRead.ikeTraceoptions[0]["file"].([]map[string]interface{})[0]["world_readable"] = true
		case strings.HasPrefix(itemTrim, "file no-world-readable"):
			confRead.ikeTraceoptions[0]["file"].([]map[string]interface{})[0]["no_world_readable"] = true
		case strings.HasPrefix(itemTrim, "file "):
			confRead.ikeTraceoptions[0]["file"].([]map[string]interface{})[0]["name"] = strings.Trim(
				strings.TrimPrefix(itemTrim, "file "), "\"")
		}
	case strings.HasPrefix(itemTrim, "flag"):
		confRead.ikeTraceoptions[0]["flag"] = append(confRead.ikeTraceoptions[0]["flag"].([]string),
			strings.TrimPrefix(itemTrim, "flag "))
	case strings.HasPrefix(itemTrim, "no-remote-trace"):
		confRead.ikeTraceoptions[0]["no_remote_trace"] = true
	case strings.HasPrefix(itemTrim, "rate-limit"):
		var err error
		confRead.ikeTraceoptions[0]["rate_limit"], err = strconv.Atoi(
			strings.TrimPrefix(itemTrim, "rate-limit "))
		if err != nil {
			return fmt.Errorf("failed to convert value from '%s' to integer : %w", itemTrim, err)
		}
	}

	return nil
}
func readSecurityAlg(confRead *securityOptions, itemTrimAlg string) {
	itemTrim := strings.TrimPrefix(itemTrimAlg, "alg ")
	if len(confRead.alg) == 0 {
		confRead.alg = append(confRead.alg, map[string]interface{}{
			"dns_disable":    false,
			"ftp_disable":    false,
			"msrpc_disable":  false,
			"pptp_disable":   false,
			"sunrpc_disable": false,
			"talk_disable":   false,
			"tftp_disable":   false,
		})
	}
	if itemTrim == "dns disable" {
		confRead.alg[0]["dns_disable"] = true
	}
	if itemTrim == "ftp disable" {
		confRead.alg[0]["ftp_disable"] = true
	}
	if itemTrim == "msrpc disable" {
		confRead.alg[0]["msrpc_disable"] = true
	}
	if itemTrim == "pptp disable" {
		confRead.alg[0]["pptp_disable"] = true
	}
	if itemTrim == "sunrpc disable" {
		confRead.alg[0]["sunrpc_disable"] = true
	}
	if itemTrim == "talk disable" {
		confRead.alg[0]["talk_disable"] = true
	}
	if itemTrim == "tftp disable" {
		confRead.alg[0]["tftp_disable"] = true
	}
}

func fillSecurity(d *schema.ResourceData, securityOptions securityOptions) {
	if tfErr := d.Set("ike_traceoptions", securityOptions.ikeTraceoptions); tfErr != nil {
		panic(tfErr)
	}
	if tfErr := d.Set("utm", securityOptions.utm); tfErr != nil {
		panic(tfErr)
	}
	if tfErr := d.Set("alg", securityOptions.alg); tfErr != nil {
		panic(tfErr)
	}
}
