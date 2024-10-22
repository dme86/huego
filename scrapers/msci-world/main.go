package main

import (
    "log"
    "net/http"
    "strings"
    "github.com/gocolly/colly"
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promhttp"
    "strconv"
)

// Define a Prometheus gauge metric to hold the MSCI World index price
var (
    msciWorldPrice = prometheus.NewGauge(prometheus.GaugeOpts{
        Name: "msci_world_last_price",
        Help: "The last recorded price of MSCI World Index from CNBC",
    })
)

func main() {
    // Register the metric with Prometheus
    prometheus.MustRegister(msciWorldPrice)

    // Start the HTTP server to serve the metrics endpoint
    go func() {
        port := ":8081"
        log.Printf("Starting server on port %s, metrics available at /metrics\n", port)
        http.Handle("/metrics", promhttp.Handler())
        log.Fatal(http.ListenAndServe(port, nil)) // Serving Prometheus metrics on port 8080
    }()

    // Start scraping the price
    scrapePrice()

    // Keep the program running indefinitely
    select {}
}

// scrapePrice scrapes the MSCI World index price from CNBC and updates the Prometheus metric
func scrapePrice() {
    // Create a new collector for scraping
    c := colly.NewCollector()

    // Look for the <span> element with the class "QuoteStrip-lastPrice" to extract the price
    c.OnHTML("span.QuoteStrip-lastPrice", func(e *colly.HTMLElement) {
        priceStr := e.Text

        // Remove commas from the price string
        priceStr = strings.Replace(priceStr, ",", "", -1)

        // Convert the price string to a float and update the Prometheus gauge
        price, err := strconv.ParseFloat(priceStr, 64)
        if err != nil {
            log.Println("Error converting price to float:", err)
            return
        }

        // Update the Prometheus gauge with the latest price
        msciWorldPrice.Set(price)
    })

    // Handle errors during scraping
    c.OnError(func(_ *colly.Response, err error) {
        log.Println("Error occurred during scraping:", err)
    })

    // Visit the CNBC page to get the MSCI World index price
    err := c.Visit("https://www.cnbc.com/quotes/.WORLD")
    if err != nil {
        log.Fatal("Failed to visit page:", err)
    }
}


