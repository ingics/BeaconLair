package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var TotalPublish = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "igs_mqtt_publish_total",
		Help: "Number of MQTT publish",
	},
	[]string{"mac"},
)

var TotalValidPublish = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "igs_mqtt_valid_publish_total",
		Help: "Number of MQTT publish",
	},
	[]string{"mac"},
)

func InitPrometheusClient() {
	port := 9000
	router := mux.NewRouter()

	prometheus.Register(TotalPublish)
	prometheus.Register(TotalValidPublish)

	router.Path("/metrics").Handler(promhttp.Handler())
	fmt.Printf("Serving prometheus requests on port %d\n", port)
	err := http.ListenAndServe(fmt.Sprintf(":%d", port), router)
	log.Fatal(err)
}
