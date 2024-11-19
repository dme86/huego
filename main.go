package main

import (
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "os"
    "sync"
    "time"

    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

// State contains the temperature value and other states for Hue sensors
type State struct {
    Temperature int    `json:"temperature"`
    LastUpdated string `json:"lastupdated"`
}

// Sensor contains the important information of a Hue sensor
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

// NewHueMetricsCollector creates a new HueMetricsCollector
func NewHueMetricsCollector(mapping map[string]string) *HueMetricsCollector {
    return &HueMetricsCollector{
        temperatureGauge: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "hue_temperature_celsius",
                Help: "Current temperature readings from Hue sensors",
            },
            []string{"sensor_name", "room"},
        ),
        sensorRoomMapping: mapping,
    }
}

// Describe sends the descriptors of each metric over to the provided channel.
func (collector *HueMetricsCollector) Describe(ch chan<- *prometheus.Desc) {
    collector.temperatureGauge.Describe(ch)
}

// Collect fetches Hue sensor data and updates the metrics
func (collector *HueMetricsCollector) Collect(ch chan<- prometheus.Metric) {
    // Retrieve Hue sensors
    sensors, err := getTemperatureSensors()
    if err != nil {
        log.Printf("Error retrieving Hue sensors: %v", err)
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

// WeatherResponse represents the structure of the Open-Meteo API response
type WeatherResponse struct {
    CurrentWeather struct {
        Temperature float64 `json:"temperature"`
        // Add more fields if needed
    } `json:"current_weather"`
}

// WeatherMetricsCollector collects the external weather temperature metric
type WeatherMetricsCollector struct {
    temperatureGauge prometheus.Gauge
    apiURL           string
    client           *http.Client
    mutex            sync.RWMutex
    cachedTemp       float64
}

// NewWeatherMetricsCollector creates a new WeatherMetricsCollector
func NewWeatherMetricsCollector(apiURL string, fetchInterval time.Duration) *WeatherMetricsCollector {
    collector := &WeatherMetricsCollector{
        temperatureGauge: prometheus.NewGauge(prometheus.GaugeOpts{
            Name: "external_temperature_celsius",
            Help: "Current external temperature in Celsius from Open-Meteo API",
        }),
        apiURL: apiURL,
        client: &http.Client{
            Timeout: 10 * time.Second, // Set a timeout for the HTTP request
        },
    }

    // Start the periodic update
    go collector.updateTemperaturePeriodically(fetchInterval)

    return collector
}

// Describe sends the descriptors of each metric over to the provided channel.
func (collector *WeatherMetricsCollector) Describe(ch chan<- *prometheus.Desc) {
    collector.temperatureGauge.Describe(ch)
}

// Collect sends the current temperature metric to Prometheus
func (collector *WeatherMetricsCollector) Collect(ch chan<- prometheus.Metric) {
    collector.mutex.RLock()
    temp := collector.cachedTemp
    collector.mutex.RUnlock()

    collector.temperatureGauge.Set(temp)
    collector.temperatureGauge.Collect(ch)
}

// updateTemperaturePeriodically fetches and updates the temperature at regular intervals
func (collector *WeatherMetricsCollector) updateTemperaturePeriodically(interval time.Duration) {
    ticker := time.NewTicker(interval)
    defer ticker.Stop()

    // Initial fetch
    collector.fetchAndUpdate()

    for range ticker.C {
        collector.fetchAndUpdate()
    }
}

// fetchAndUpdate fetches the temperature from the API and updates the cached value
func (collector *WeatherMetricsCollector) fetchAndUpdate() {
    resp, err := collector.client.Get(collector.apiURL)
    if err != nil {
        log.Printf("Error fetching weather data: %v", err)
        return
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        log.Printf("Unexpected status code from weather API: %d", resp.StatusCode)
        return
    }

    var weather WeatherResponse
    if err := json.NewDecoder(resp.Body).Decode(&weather); err != nil {
        log.Printf("Error decoding weather data: %v", err)
        return
    }

    temp := weather.CurrentWeather.Temperature

    collector.mutex.Lock()
    collector.cachedTemp = temp
    collector.mutex.Unlock()

    log.Printf("Updated external temperature: %.2f°C", temp)
}

func main() {
    // ------------------ Hue Metrics Setup ------------------
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

    hueCollector := NewHueMetricsCollector(sensorRoomMapping)

    // ------------------ Weather Metrics Setup ------------------
    // Read latitude and longitude from environment variables or set default values
    latitude := getEnv("WEATHER_LATITUDE", "50.7374")
    longitude := getEnv("WEATHER_LONGITUDE", "7.0982")

    // Construct the Open-Meteo API URL
    weatherAPIURL := fmt.Sprintf("https://api.open-meteo.com/v1/forecast?latitude=%s&longitude=%s&current_weather=true", latitude, longitude)

    // Fetch interval for weather data (e.g., every 5 minutes)
    fetchIntervalStr := getEnv("WEATHER_FETCH_INTERVAL", "5m")
    fetchInterval, err := time.ParseDuration(fetchIntervalStr)
    if err != nil {
        log.Fatalf("Invalid WEATHER_FETCH_INTERVAL: %v", err)
    }

    weatherCollector := NewWeatherMetricsCollector(weatherAPIURL, fetchInterval)

    // ------------------ Prometheus Metrics Registration ------------------
    // Register Prometheus collectors
    prometheus.MustRegister(hueCollector)
    prometheus.MustRegister(weatherCollector)

    // HTTP handler for Prometheus metrics
    http.Handle("/metrics", promhttp.Handler())

    // Start the exporter on port 8000 or as defined in environment
    port := getEnv("EXPORTER_PORT", "8000")
    log.Printf("Starting Exporter on port %s...", port)
    if err := http.ListenAndServe(":"+port, nil); err != nil {
        log.Fatalf("Error starting HTTP server: %v", err)
    }
}

// getEnv reads an environment variable or returns a default value if not set
func getEnv(key, defaultVal string) string {
    if value, exists := os.LookupEnv(key); exists {
        return value
    }
    return defaultVal
}

