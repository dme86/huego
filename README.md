# Huego

**Huego** is a Prometheus exporter for Philips Hue sensors, written in Go. It collects temperature readings from Hue devices and exposes them as Prometheus metrics for easy integration into your monitoring stack.
Phase: Early Alpha

## Features

- Collects temperature readings from Philips Hue sensors.
- Exposes temperature metrics in a format compatible with Prometheus.
- Lightweight and easy to run in Kubernetes or standalone environments.
- Customizable via environment variables for IP address and API key.

## Installation

To get started with Huego, clone the repository and build the Go application:

```bash
git clone https://github.com/yourusername/huego.git
cd huego
go build
```

## Usage

Make sure you have the Philips Hue API key and the IP address of your Hue Bridge.

Export the environment variables:
```shell
export HUE_BRIDGE_IP=<your-hue-bridge-ip>
export HUE_API_KEY=<your-api-key>
```

Run the exporter:
```shell
./huego
```

The application will expose metrics at:
```
http://localhost:8000/metrics
```

Example output:
```
hue_temperature{sensor_name="Hue temperature sensor 3"} 19.65
hue_temperature{sensor_name="Hue temperature sensor 4"} 21.38
hue_temperature{sensor_name="Hue temperature sensor 7"} 22.69
hue_temperature{sensor_name="Hue temperature sensor 8"} 20.87
hue_temperature{sensor_name="Hue temperature sensor 9"} 22.87
```
## Deployment with Docker

You can also run Huego using Docker. First, build the Docker image:
```
docker build -t huego .
```

Run the Docker container:
```
docker run -e HUE_BRIDGE_IP=<your-hue-bridge-ip> -e HUE_API_KEY=<your-api-key> -p 8000:8000 huego
```

## Deployment on Kubernetes

If you are running Kubernetes (e.g., with k3d), deploy Huego as a pod or service, making sure the environment variables are properly configured in your manifest.

## License

This project is licensed under the MIT License. See the LICENSE file for details.
