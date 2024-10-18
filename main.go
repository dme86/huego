package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// State contains the temperature value and other states
type State struct {
	Temperature int    `json:"temperature"`
	LastUpdated string `json:"lastupdated"`
}

// Sensor contains the important information of a sensor
type Sensor struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	State State  `json:"state"`
}

// HueSensors contains the response from the Hue API
type HueSensors map[string]Sensor

// HueMetricsCollector collects the Hue sensor metrics
type HueMetricsCollector struct {
	temperatureGauge *prometheus.GaugeVec
}

func NewHueMetricsCollector() *HueMetricsCollector {
	return &HueMetricsCollector{
		temperatureGauge: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "hue_temperature",
				Help: "Current temperature readings from Hue sensors",
			},
			[]string{"sensor_name"},
		),
	}
}

func (collector *HueMetricsCollector) Describe(ch chan<- *prometheus.Desc) {
	collector.temperatureGauge.Describe(ch)
}

func (collector *HueMetricsCollector) Collect(ch chan<- prometheus.Metric) {
	// Retrieve Hue sensors
	sensors, err := getTemperatureSensors()
	if err != nil {
		log.Printf("Error retrieving sensors: %v", err)
		return
	}

	// Update the metrics
	for _, sensor := range sensors {
		if sensor.Type == "ZLLTemperature" {
			temperatureCelsius := float64(sensor.State.Temperature) / 100.0
			collector.temperatureGauge.WithLabelValues(sensor.Name).Set(temperatureCelsius)
		}
	}

	collector.temperatureGauge.Collect(ch)
}

// Retrieve Hue sensors from the bridge
func getTemperatureSensors() ([]Sensor, error) {
	hueBridgeIP := os.Getenv("HUE_BRIDGE_IP")
	apiKey := os.Getenv("HUE_API_KEY")

	if hueBridgeIP == "" || apiKey == "" {
		return nil, fmt.Errorf("HUE_BRIDGE_IP or HUE_API_KEY environment variable not set")
	}

	url := fmt.Sprintf("http://%s/api/%s/sensors", hueBridgeIP, apiKey)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var sensors HueSensors
	if err := json.NewDecoder(resp.Body).Decode(&sensors); err != nil {
		return nil, err
	}

	var temperatureSensors []Sensor
	for _, sensor := range sensors {
		if sensor.Type == "ZLLTemperature" {
			temperatureSensors = append(temperatureSensors, sensor)
		}
	}

	return temperatureSensors, nil
}

func main() {
	collector := NewHueMetricsCollector()

	// Register Prometheus collector
	prometheus.MustRegister(collector)

	// HTTP handler for Prometheus metrics
	http.Handle("/metrics", promhttp.Handler())

	// Start the exporter on port 8000
	log.Println("Starting Hue Exporter on port 8000...")
	log.Fatal(http.ListenAndServe(":8000", nil))
}

