package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// helper: JSON-to-Go (https://mholt.github.io/json-to-go/)

/* helper function for debug only */
func printRequest(c *gin.Context) {
	log.Printf("%s\n", c.Request.Header)
	buf := make([]byte, 1024)
	num, _ := c.Request.Body.Read(buf)
	reqBody := string(buf[0:num])
	log.Printf("%s\n", reqBody)
	c.Status(http.StatusNotImplemented)
}

func formatGatewayInfo(gw *GatewayEntry) map[string]interface{} {
	return map[string]interface{}{
		"Model":     gw.Info.Model,
		"Fw":        gw.Info.FwVer,
		"Mac":       gw.Info.Mac,
		"BleFw":     gw.Info.BleFw,
		"BleMac":    gw.Info.BleMac,
		"Network":   gw.NetworkDesc(),
		"UpTime":    gw.UpTimeDesc(),
		"Connected": gw.Connected(),
		"Identity":  gw.Info.Identity(),
	}
}

func handleGatewayList(c *gin.Context) {
	gws := map[string]map[string]interface{}{}
	for _, gw := range gatewayList {
		gws[gw.Info.Token] = formatGatewayInfo(gw)
	}
	c.IndentedJSON(http.StatusOK, gws)
}

func handleGatewayGet(c *gin.Context) {
	if token, ok := c.Params.Get("token"); ok {
		if gw, ok := gatewayList[token]; ok {
			data := formatGatewayInfo(gw)
			data["RssiThreshold"] = -100
			if resp, err := gw.execCmd("BLE RSSITHR", time.Second*3); err == nil {
				if strval, ok := resp.data["BLE RSSITHR"]; ok {
					if val, err := strconv.Atoi(strval.(string)); err == nil {
						data["RssiThreshold"] = val
					}
				}
			}
			c.IndentedJSON(http.StatusOK, data)
		} else {
			c.Status(http.StatusNotFound)
		}
	} else {
		// for Grafana usage,
		// return empty object instead of error
		c.IndentedJSON(http.StatusOK, map[string]interface{}{})
	}
}

type GatewayPutParam struct {
	RssiThreshold int `json:"rssi"`
}

func handleGatewayPut(c *gin.Context) {
	if token, ok := c.Params.Get("token"); ok {
		if gw, ok := gatewayList[token]; ok {
			var param GatewayPutParam
			if err := c.ShouldBindJSON(&param); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			if param.RssiThreshold != 0 {
				if param.RssiThreshold < -127 || param.RssiThreshold >= 0 {
					c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid RSSI threshold"})
					return
				}
				cmd := fmt.Sprintf("BLE RSSITHR %d", param.RssiThreshold)
				if resp, err := gw.execCmd(cmd, time.Second*5); err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				} else {
					if resp.result != 0 {
						c.JSON(
							http.StatusInternalServerError,
							gin.H{"error": fmt.Sprintf("RESULT: %d", resp.result)})
					}
				}
			}
			c.JSON(http.StatusOK, gin.H{"Message": "OK"})
		} else {
			c.Status(http.StatusNotFound)
		}
	} else {
		c.JSON(
			http.StatusBadGateway,
			gin.H{"error": "Token is required"},
		)
	}
}

func handleGatewayOTA(c *gin.Context) {
	if token, ok := c.Params.Get("token"); ok {
		if gw, ok := gatewayList[token]; ok {
			if resp, err := gw.execCmd("SYS OTA START", time.Second*10); err == nil {
				c.JSON(http.StatusOK, gin.H{"Message": resp.lines[0]})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			}
		} else {
			c.Status(http.StatusNotFound)
		}
	} else {
		c.JSON(
			http.StatusBadGateway,
			gin.H{"error": "Token is required"},
		)
	}
}

func (gw *GatewayEntry) autoset() error {
	host, err := gw.getRCHost()
	if err != nil {
		return fmt.Errorf("fail to get CTRLHOST")
	}
	cmdList := []string{
		"BLE PAYLOADWL 1 020106XXFFXX008XBC", // IBS01/02/03/04
		"BLE PAYLOADWL 2 020106XXFF2C088XBC", // IBS05/06
		fmt.Sprintf("MQTT HOST %s", host),
		fmt.Sprintf("MQTT PORT %d", getEnvIntVar("MQTT_PORT", 1883)),
		"MQTT USERNAME beaconlair",
		"MQTT PASSWORD beaconlair",
		"MQTT PUBTOPIC pub",
		"MQTT FORMAT 0",
		"SYS WORKMODE 3", // MQTT Client
	}
	for _, cmd := range cmdList {
		if _, err := gw.execCmd(cmd, time.Second*3); err != nil {
			return fmt.Errorf("cmd: %s, error: %s", cmd, err)
		}
	}
	return nil
}

func handleGatewayAutoSet(c *gin.Context) {
	if token, ok := c.Params.Get("token"); ok {
		if gw, ok := gatewayList[token]; ok {
			if err := gw.autoset(); err == nil {
				c.JSON(http.StatusOK, gin.H{"Message": "OK"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			}
		} else {
			c.Status(http.StatusNotFound)
		}
	} else {
		c.JSON(
			http.StatusBadGateway,
			gin.H{"error": "Token is required"},
		)
	}
}

// Initialize a HTTP server for Grafana JSON data source plugin
// (https://grafana.com/grafana/plugins/simpod-json-datasource/)
// and provide gateway remote control API.
func InitHttpServer() {
	server := gin.Default()
	server.Use(cors.Default())
	// Gateway remote control APIs
	server.GET("/gateways", handleGatewayList)
	server.GET("/gateways/:token", handleGatewayGet)
	server.PUT("/gateways/:token", handleGatewayPut)
	server.PUT("/gateways/:token/actions/ota", handleGatewayOTA)
	server.PUT("/gateways/:token/actions/autoset", handleGatewayAutoSet)
	// WK for Grafana:
	// If no gateway found in list, the Grafana Panel will query
	// gateway info with empty token.
	// Handle this case specially to avoid error message on Grafana Panel
	server.RedirectTrailingSlash = false
	server.GET("/gateways/", handleGatewayGet) // WK for Grafana
	// Start the server
	server.Run(":5001")
}
