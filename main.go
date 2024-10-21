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
    temperatureGauge  *prometheus.GaugeVec
    sensorRoomMapping map[string]string
}

func NewHueMetricsCollector(mapping map[string]string) *HueMetricsCollector {
    return &HueMetricsCollector{
        temperatureGauge: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "hue_temperature",
                Help: "Current temperature readings from Hue sensors",
            },
            []string{"sensor_name", "room"},
        ),
        sensorRoomMapping: mapping,
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

            // Get room name from mapping
            roomName, ok := collector.sensorRoomMapping[sensor.Name]
            if !ok {
                // If the sensor name is not in the mapping, use a default value
                roomName = "Unknown"
            }

            collector.temperatureGauge.With(prometheus.Labels{
                "sensor_name": sensor.Name,
                "room":        roomName,
            }).Set(temperatureCelsius)
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
    // Read the sensor-room mapping from environment variable
    mappingJSON := os.Getenv("SENSOR_ROOM_MAPPING")
    if mappingJSON == "" {
        log.Fatal("SENSOR_ROOM_MAPPING environment variable not set")
    }

    // Parse the JSON mapping
    var sensorRoomMapping map[string]string
    if err := json.Unmarshal([]byte(mappingJSON), &sensorRoomMapping); err != nil {
        log.Fatalf("Error parsing SENSOR_ROOM_MAPPING: %v", err)
    }

    collector := NewHueMetricsCollector(sensorRoomMapping)

    // Register Prometheus collector
    prometheus.MustRegister(collector)

    // HTTP handler for Prometheus metrics
    http.Handle("/metrics", promhttp.Handler())

    // Start the exporter on port 8000
    log.Println("Starting Hue Exporter on port 8000...")
    log.Fatal(http.ListenAndServe(":8000", nil))
}

