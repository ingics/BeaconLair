package main

import "time"

func main() {

	go InitInfluxDBClient()
	go InitializeMQTTClient()
	go InitPrometheusClient()

	for {
		time.Sleep(time.Second * 3)
	}
}
